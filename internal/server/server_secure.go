package server

import (
	"crypto/rand"
	"encoding/json"
	"net"
	"time"

	"darksideprotocol/internal/secureproto"
	"darksideprotocol/internal/transport/tcp"
)

func (s *Server) handleTCPPayload(conn net.Conn, payload []byte) {
	if len(payload) == 0 {
		s.failConn(conn, "empty frame")
		return
	}
	if s.cfg.ServerPrivateKey == "" {
		if len(payload) > s.cfg.MaxPacketSize {
			s.failConn(conn, "frame too large")
			return
		}
		_ = tcp.WriteFrame(conn, s.decorateReply(payload))
		return
	}
	if len(payload) > s.cfg.MaxPacketSize*8 {
		s.failConn(conn, "frame too large")
		return
	}

	state := s.getConnState(conn)
	state.lastSeen = time.Now()
	if !state.secureReady {
		s.handleHelloFrame(conn, payload, state)
		return
	}
	s.handleDataFrame(conn, payload, state)
}

func (s *Server) handleHelloFrame(conn net.Conn, payload []byte, state *connState) {
	hello, clientNonce, err := secureproto.ParseHello(payload)
	if err != nil {
		s.failConn(conn, "invalid hello")
		return
	}
	if !secureproto.ValidateTimestamp(hello.Timestamp, int64(s.cfg.HandshakeSkewSec)) {
		s.failConn(conn, "stale hello timestamp")
		return
	}
	if !secureproto.VerifyHelloAuth(s.cfg.PreSharedKey, hello, clientNonce) {
		s.failConn(conn, "hello auth failed")
		return
	}
	shared, err := secureproto.SharedSecret(s.cfg.ServerPrivateKey, hello.ClientPublicKey)
	if err != nil {
		s.failConn(conn, "handshake failed")
		return
	}

	serverNonce := make([]byte, 16)
	if _, err := rand.Read(serverNonce); err != nil {
		s.failConn(conn, "nonce failed")
		return
	}
	state.sessionKey = secureproto.DeriveSessionKey(shared, clientNonce, serverNonce)
	state.secureReady = true
	ack := secureproto.BuildAck(state.sessionKey, serverNonce)
	raw, _ := json.Marshal(ack)
	if err := tcp.WriteFrame(conn, raw); err != nil {
		s.removeConnState(conn)
	}
}

func (s *Server) handleDataFrame(conn net.Conn, payload []byte, state *connState) {
	var req secureproto.DataFrame
	if err := json.Unmarshal(payload, &req); err != nil || req.Type != secureproto.TypeData {
		s.failConn(conn, "bad secure frame")
		return
	}
	plain, err := secureproto.Decrypt(state.sessionKey, req.Ciphertext)
	if err != nil {
		s.failConn(conn, "decrypt failed")
		return
	}
	if len(plain) > s.cfg.MaxPacketSize {
		s.failConn(conn, "payload too large")
		return
	}
	reply := s.decorateReply(plain)
	enc, err := secureproto.Encrypt(state.sessionKey, reply)
	if err != nil {
		s.failConn(conn, "encrypt failed")
		return
	}
	resp, _ := json.Marshal(secureproto.DataFrame{
		Type:       secureproto.TypeData,
		Ciphertext: enc,
	})
	if err := tcp.WriteFrame(conn, resp); err != nil {
		s.removeConnState(conn)
	}
}
