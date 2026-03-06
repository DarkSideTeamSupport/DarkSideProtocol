package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"darksideprotocol/internal/config"
	"darksideprotocol/internal/policy"
	"darksideprotocol/internal/secureproto"
	"darksideprotocol/internal/transport/tcp"
	"darksideprotocol/internal/transport/udp"
)

type Client struct {
	cfg       config.ClientConfig
	policy    *policy.Engine
	sessionID string
	seq       uint32
}

func New(cfg config.ClientConfig) (*Client, error) {
	mode := policy.ModeUDP
	if strings.EqualFold(cfg.TransportMode, string(policy.ModeTCP)) {
		mode = policy.ModeTCP
	}

	return &Client{
		cfg:    cfg,
		policy: policy.New(mode),
	}, nil
}

func (c *Client) Run(ctx context.Context) error {
	for {
		if err := c.runSession(ctx); err != nil {
			log.Printf("session error mode=%s err=%v", c.policy.Current(), err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(2 * time.Second):
		}
	}
}

func (c *Client) runSession(ctx context.Context) error {
	tc, err := tcp.Dial(c.cfg.ServerTCP)
	if err != nil {
		return err
	}
	defer tc.Close()

	sessionKey, err := c.doHandshake(tc)
	if err != nil {
		return err
	}

	var uc *udp.Client
	if c.cfg.EnableMultiTransport && c.cfg.ServerUDP != "" && c.sessionID != "" {
		uc, _ = udp.Dial(c.cfg.ServerUDP)
		if uc != nil {
			defer uc.Close()
			_ = c.bindUDPPlane(uc, sessionKey)
		}
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			msg := []byte(fmt.Sprintf("v2-probe ts=%d", time.Now().Unix()))
			start := time.Now()
			reply, usedUDP, err := c.request(tc, uc, sessionKey, msg)
			if err != nil {
				c.policy.Observe(false, 0)
				return err
			}
			c.policy.Observe(true, time.Since(start))
			log.Printf("reply mode=%s profile=%s via_udp=%v: %s", c.policy.Current(), c.policy.Profile(), usedUDP, string(reply))
		}
	}
}

func (c *Client) doHandshake(tc *tcp.Client) ([]byte, error) {
	hello, clientNonce, err := secureproto.NewHello(c.cfg.ClientPublicKey, c.cfg.PreSharedKey, c.cfg.ProtocolVersion)
	if err != nil {
		return nil, err
	}
	rawHello, _ := secureproto.HelloJSON(hello)
	if err := tc.Send(rawHello); err != nil {
		return nil, err
	}
	resp, err := tc.Receive(5 * time.Second)
	if err != nil {
		return nil, err
	}

	if c.cfg.ProtocolVersion == "v2" {
		challenge, serverNonce, err := secureproto.ParseChallenge(resp)
		if err != nil {
			return nil, err
		}
		shared, err := secureproto.SharedSecret(c.cfg.ClientPrivateKey, c.cfg.ServerPublicKey)
		if err != nil {
			return nil, err
		}
		sessionKey := secureproto.DeriveSessionKey(shared, clientNonce, serverNonce)
		auth := secureproto.AuthFrame{
			Type:         secureproto.TypeAuth,
			ProtoVersion: "v2",
			Proof:        secureproto.BuildAuthProof(sessionKey, challenge.Ticket),
		}
		rawAuth, _ := json.Marshal(auth)
		if err := tc.Send(rawAuth); err != nil {
			return nil, err
		}
		readyRaw, err := tc.Receive(5 * time.Second)
		if err != nil {
			return nil, err
		}
		var ready secureproto.ReadyFrame
		if err := json.Unmarshal(readyRaw, &ready); err != nil {
			return nil, err
		}
		if !secureproto.VerifyReady(sessionKey, ready) {
			return nil, fmt.Errorf("ready_v2 verification failed")
		}
		c.sessionID = ready.SessionID
		return sessionKey, nil
	}

	var ack secureproto.AckFrame
	if err := json.Unmarshal(resp, &ack); err != nil {
		return nil, err
	}
	serverNonce, err := decodeB64(ack.ServerNonce)
	if err != nil {
		return nil, err
	}
	shared, err := secureproto.SharedSecret(c.cfg.ClientPrivateKey, c.cfg.ServerPublicKey)
	if err != nil {
		return nil, err
	}
	sessionKey := secureproto.DeriveSessionKey(shared, clientNonce, serverNonce)
	if err := secureproto.VerifyAck(sessionKey, ack); err != nil {
		return nil, err
	}
	return sessionKey, nil
}

func (c *Client) bindUDPPlane(uc *udp.Client, sessionKey []byte) error {
	c.seq++
	raw, err := secureproto.BuildDatagramFrameV2(sessionKey, c.sessionID, 2, c.seq, "bind", []byte("bind"), 24)
	if err != nil {
		return err
	}
	if err := uc.Send(raw); err != nil {
		return err
	}
	_, _ = uc.Receive(2 * time.Second)
	return nil
}

func (c *Client) request(tc *tcp.Client, uc *udp.Client, sessionKey []byte, plain []byte) ([]byte, bool, error) {
	c.seq++
	mode := secureproto.SelectObfsModeForProfile(c.policy.Profile(), c.seq, len(plain))
	useUDP := c.cfg.ProtocolVersion == "v2" && c.policy.Current() == policy.ModeUDP && uc != nil && c.sessionID != ""

	if useUDP {
		raw, err := secureproto.BuildDatagramFrameV2(sessionKey, c.sessionID, 2, c.seq, mode, plain, 48)
		if err == nil && uc.Send(raw) == nil {
			replyRaw, err := uc.Receive(3 * time.Second)
			if err == nil {
				_, payload, err := secureproto.ParseDatagramFrameV2(sessionKey, replyRaw)
				if err == nil {
					return payload, true, nil
				}
			}
		}
		c.policy.Observe(false, 0)
	}

	raw, err := secureproto.BuildDataFrameV2(sessionKey, 2, c.seq, mode, plain, 48)
	if err != nil {
		return nil, false, err
	}
	if err := tc.Send(raw); err != nil {
		return nil, false, err
	}
	replyRaw, err := tc.Receive(3 * time.Second)
	if err != nil {
		return nil, false, err
	}
	_, payload, err := secureproto.ParseDataFrameV2(sessionKey, replyRaw)
	if err != nil {
		return nil, false, err
	}
	return payload, false, nil
}

func decodeB64(v string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(v)
}

func (c *Client) CurrentMode() string {
	return strings.ToUpper(string(c.policy.Current()))
}

func (c *Client) String() string {
	return fmt.Sprintf("client{proto=%s mode=%s}", c.cfg.ProtocolVersion, c.CurrentMode())
}

func (c *Client) Close() error {
	return nil
}

func (c *Client) Validate() error {
	if c.cfg.ServerTCP == "" {
		return fmt.Errorf("server_tcp is empty")
	}
	if c.cfg.PreSharedKey == "" {
		return fmt.Errorf("pre_shared_key is empty")
	}
	if c.cfg.ServerPublicKey == "" || c.cfg.ClientPrivateKey == "" || c.cfg.ClientPublicKey == "" {
		return fmt.Errorf("x25519 keys are required for secure protocol")
	}
	return nil
}
