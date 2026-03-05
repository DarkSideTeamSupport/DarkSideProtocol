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
	hello, clientNonce, err := secureproto.NewHello(c.cfg.ClientPublicKey, c.cfg.PreSharedKey)
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
	if err := c.sendEncryptedPayload(conn, sessionKey, plain); err != nil {
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
	defer dev.Close()

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

	errCh := make(chan error, 2)
	go func() {
		for {
			pkt, err := dev.ReadPacket()
			if err != nil {
				errCh <- err
				return
			}
			if len(pkt) == 0 {
				continue
			}
			if err := c.sendEncryptedPayload(conn, sessionKey, pkt); err != nil {
				errCh <- err
				return
			}
		}
	}()

	go func() {
		for {
			pkt, err := c.readEncryptedPayload(conn, sessionKey, 60*time.Second)
			if err != nil {
				errCh <- err
				return
			}
			if len(pkt) == 0 {
				continue
			}
			if err := dev.WritePacket(pkt); err != nil {
				errCh <- err
				return
			}
		}
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

func (c *SecureTCPClient) sendEncryptedPayload(conn net.Conn, sessionKey []byte, plain []byte) error {
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
