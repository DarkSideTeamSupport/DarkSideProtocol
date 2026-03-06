package winclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"darksideprotocol/internal/secureproto"
	"darksideprotocol/internal/transport/tcp"
	"darksideprotocol/internal/transport/udp"
)

type SecureTCPClient struct {
	cfg      Config
	sessionID string
	udpClient *udp.Client
	seqCounter uint32
}

func NewSecureTCPClient(cfg Config) *SecureTCPClient {
	return &SecureTCPClient{cfg: cfg}
}

func (c *SecureTCPClient) Run(ctx context.Context) error {
	for {
		if err := c.runOneSession(ctx); err != nil {
			log.Printf("session error: %v", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Duration(c.cfg.ReconnectSec) * time.Second):
		}
	}
}

func (c *SecureTCPClient) runOneSession(ctx context.Context) error {
	conn, err := net.DialTimeout("tcp", c.cfg.ServerTCP, time.Duration(c.cfg.HandshakeTimeout)*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	sessionKey, err := c.doHandshake(conn)
	if err != nil {
		return err
	}
	c.closeUDP()
	if err := c.initUDPPlane(sessionKey); err != nil {
		log.Printf("udp plane disabled: %v", err)
	}
	defer c.closeUDP()
	log.Printf("secure session established with %s", c.cfg.ServerTCP)
	if c.cfg.EnableTunnel {
		return c.runTunnelSession(ctx, conn, sessionKey)
	}
	ticker := time.NewTicker(time.Duration(c.cfg.PingIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			msg := fmt.Sprintf("profile=%s device=%s ts=%d", c.cfg.ProfileName, c.cfg.DeviceName, time.Now().Unix())
			if err := c.secureRequest(conn, sessionKey, []byte(msg)); err != nil {
				return err
			}
		}
	}
}

func (c *SecureTCPClient) doHandshake(conn net.Conn) ([]byte, error) {
	hello, clientNonce, err := secureproto.NewHello(c.cfg.ClientPublicKey, c.cfg.PreSharedKey, c.cfg.ProtocolVersion)
	if err != nil {
		return nil, err
	}
	rawHello, _ := secureproto.HelloJSON(hello)
	if err := tcp.WriteFrame(conn, rawHello); err != nil {
		return nil, err
	}

	payload, err := readFrameWithTimeout(conn, time.Duration(c.cfg.HandshakeTimeout)*time.Second)
	if err != nil {
		return nil, err
	}
	var ack secureproto.AckFrame
	if c.cfg.ProtocolVersion == "v2" {
		return c.doHandshakeV2(conn, payload, clientNonce)
	}
	if err := json.Unmarshal(payload, &ack); err != nil {
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

func (c *SecureTCPClient) doHandshakeV2(conn net.Conn, firstPayload []byte, clientNonce []byte) ([]byte, error) {
	challenge, serverNonce, err := secureproto.ParseChallenge(firstPayload)
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
	if err := tcp.WriteFrame(conn, rawAuth); err != nil {
		return nil, err
	}

	rawReady, err := readFrameWithTimeout(conn, time.Duration(c.cfg.HandshakeTimeout)*time.Second)
	if err != nil {
		return nil, err
	}
	var ready secureproto.ReadyFrame
	if err := json.Unmarshal(rawReady, &ready); err != nil {
		return nil, err
	}
	if !secureproto.VerifyReady(sessionKey, ready) {
		return nil, fmt.Errorf("ready_v2 verification failed")
	}
	c.sessionID = ready.SessionID
	return sessionKey, nil
}

func (c *SecureTCPClient) secureRequest(conn net.Conn, sessionKey []byte, plain []byte) error {
	if err := c.sendEncryptedPayload(conn, sessionKey, plain, 2); err != nil {
		return err
	}
	reply, err := c.readEncryptedPayload(conn, sessionKey, time.Duration(c.cfg.HandshakeTimeout)*time.Second)
	if err != nil {
		return err
	}
	log.Printf("server reply: %s", string(reply))
	return nil
}

func (c *SecureTCPClient) runTunnelSession(ctx context.Context, conn net.Conn, sessionKey []byte) error {
	dev, err := openTunnelDevice(c.cfg.TunName)
	if err != nil {
		return err
	}
	sessionCtx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		_ = dev.Close()
	}()

	name, err := dev.Name()
	if err != nil {
		return err
	}
	if err := configureTunnelInterface(name, c.cfg.TunCIDR, c.cfg.TunGateway, c.cfg.TunSetDefaultRoute); err != nil {
		return err
	}
	log.Printf("tunnel interface ready: %s (%s)", name, c.cfg.TunCIDR)
	if c.cfg.TunProbeOnly {
		log.Printf("tunnel probe mode enabled (dataplane disabled, default route switch=%v)", c.cfg.TunSetDefaultRoute)
		ticker := time.NewTicker(time.Duration(c.cfg.PingIntervalSec) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				msg := fmt.Sprintf("tunnel-probe profile=%s device=%s ts=%d", c.cfg.ProfileName, c.cfg.DeviceName, time.Now().Unix())
				if err := c.secureRequest(conn, sessionKey, []byte(msg)); err != nil {
					return err
				}
			}
		}
	}

	errCh := make(chan error, 3)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-sessionCtx.Done():
				return
			default:
			}
			pkt, err := dev.ReadPacket()
			if err != nil {
				if sessionCtx.Err() != nil {
					return
				}
				errCh <- err
				return
			}
			if len(pkt) == 0 {
				continue
			}
			if err := c.sendEncryptedPayload(conn, sessionKey, pkt, 1); err != nil {
				if sessionCtx.Err() != nil {
					return
				}
				errCh <- err
				return
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-sessionCtx.Done():
				return
			default:
			}
			pkt, err := c.readEncryptedPayload(conn, sessionKey, 60*time.Second)
			if err != nil {
				if sessionCtx.Err() != nil {
					return
				}
				if ne, ok := err.(net.Error); ok && ne.Timeout() {
					continue
				}
				errCh <- err
				return
			}
			if len(pkt) == 0 {
				continue
			}
			if err := dev.WritePacket(pkt); err != nil {
				if sessionCtx.Err() != nil {
					return
				}
				errCh <- err
				return
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-sessionCtx.Done():
				return
			default:
			}
			if c.udpClient == nil || c.sessionID == "" {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			raw, err := c.udpClient.Receive(3 * time.Second)
			if err != nil {
				if ne, ok := err.(net.Error); ok && ne.Timeout() {
					continue
				}
				if sessionCtx.Err() != nil {
					return
				}
				continue
			}
			frame, payload, err := secureproto.ParseDatagramFrameV2(sessionKey, raw)
			if err != nil {
				continue
			}
			if frame.SessionID != c.sessionID || frame.Channel != 1 {
				continue
			}
			if len(payload) == 0 {
				continue
			}
			if err := dev.WritePacket(payload); err != nil {
				if sessionCtx.Err() != nil {
					return
				}
				errCh <- err
				return
			}
		}
	}()

	select {
	case <-ctx.Done():
		cancel()
		wg.Wait()
		return nil
	case err := <-errCh:
		cancel()
		wg.Wait()
		return err
	}
}

func (c *SecureTCPClient) sendEncryptedPayload(conn net.Conn, sessionKey []byte, plain []byte, channel uint8) error {
	if c.cfg.ProtocolVersion == "v2" {
		seq := c.nextSeq()
		mode := secureproto.SelectObfsMode(seq, len(plain))
		raw, err := secureproto.BuildDataFrameV2(sessionKey, channel, seq, mode, plain, 64)
		if err != nil {
			return err
		}
		if channel == 1 && c.shouldUseUDP(seq) && c.udpClient != nil && c.sessionID != "" {
			dgram, err := secureproto.BuildDatagramFrameV2(sessionKey, c.sessionID, channel, seq, mode, plain, 64)
			if err == nil {
				if err := c.udpClient.Send(dgram); err == nil {
					return nil
				}
			}
		}
		return tcp.WriteFrame(conn, raw)
	}
	ciphertext, err := secureproto.Encrypt(sessionKey, plain)
	if err != nil {
		return err
	}
	req, _ := json.Marshal(secureproto.DataFrame{
		Type:       secureproto.TypeData,
		Ciphertext: ciphertext,
	})
	if err := tcp.WriteFrame(conn, req); err != nil {
		return err
	}
	return nil
}

func (c *SecureTCPClient) readEncryptedPayload(conn net.Conn, sessionKey []byte, timeout time.Duration) ([]byte, error) {
	respRaw, err := readFrameWithTimeout(conn, timeout)
	if err != nil {
		return nil, err
	}
	if c.cfg.ProtocolVersion == "v2" {
		_, payload, err := secureproto.ParseDataFrameV2(sessionKey, respRaw)
		return payload, err
	}
	var resp secureproto.DataFrame
	if err := json.Unmarshal(respRaw, &resp); err != nil {
		return nil, err
	}
	reply, err := secureproto.Decrypt(sessionKey, resp.Ciphertext)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

func (c *SecureTCPClient) shouldUseUDP(seq uint32) bool {
	if !c.cfg.EnableMultiTransport {
		return false
	}
	if c.cfg.ServerUDP == "" {
		return false
	}
	return seq%4 != 0
}

func (c *SecureTCPClient) nextSeq() uint32 {
	return atomic.AddUint32(&c.seqCounter, 1)
}

func (c *SecureTCPClient) initUDPPlane(sessionKey []byte) error {
	if c.cfg.ProtocolVersion != "v2" || !c.cfg.EnableMultiTransport || c.cfg.ServerUDP == "" || c.sessionID == "" {
		return nil
	}
	uc, err := udp.Dial(c.cfg.ServerUDP)
	if err != nil {
		return err
	}
	c.udpClient = uc
	seq := c.nextSeq()
	raw, err := secureproto.BuildDatagramFrameV2(sessionKey, c.sessionID, 2, seq, "bind", []byte("bind"), 24)
	if err != nil {
		return err
	}
	return c.udpClient.Send(raw)
}

func (c *SecureTCPClient) closeUDP() {
	if c.udpClient != nil {
		_ = c.udpClient.Close()
		c.udpClient = nil
	}
}
