package winclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"

	"darksideprotocol/internal/secureproto"
	"darksideprotocol/internal/transport/tcp"
)

type SecureTCPClient struct {
	cfg Config
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
	log.Printf("secure session established with %s", c.cfg.ServerTCP)
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
	hello, clientNonce, err := secureproto.NewHello(c.cfg.ClientPublicKey)
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

func (c *SecureTCPClient) secureRequest(conn net.Conn, sessionKey []byte, plain []byte) error {
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
	respRaw, err := readFrameWithTimeout(conn, time.Duration(c.cfg.HandshakeTimeout)*time.Second)
	if err != nil {
		return err
	}
	var resp secureproto.DataFrame
	if err := json.Unmarshal(respRaw, &resp); err != nil {
		return err
	}
	reply, err := secureproto.Decrypt(sessionKey, resp.Ciphertext)
	if err != nil {
		return err
	}
	log.Printf("server reply: %s", string(reply))
	return nil
}
