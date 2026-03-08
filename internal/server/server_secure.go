package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log"
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
		s.handlePreSecureFrame(conn, payload, state)
		return
	}
	s.handleDataFrame(conn, payload, state)
}

func (s *Server) handlePreSecureFrame(conn net.Conn, payload []byte, state *connState) {
	if state.awaitAuthV2 {
		s.handleAuthV2Frame(conn, payload, state)
		return
	}
	s.handleHelloFrame(conn, payload, state)
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
	state.conn = conn
	state.protoVersion = hello.ProtoVersion

	if hello.ProtoVersion == "v2" {
		challenge := secureproto.BuildChallenge(state.sessionKey, serverNonce)
		raw, _ := json.Marshal(challenge)
		state.pendingTicket = challenge.Ticket
		state.awaitAuthV2 = true
		if err := tcp.WriteFrame(conn, raw); err != nil {
			s.removeConnState(conn)
		}
		return
	}

	state.secureReady = true
	ack := secureproto.BuildAck(state.sessionKey, serverNonce)
	raw, _ := json.Marshal(ack)
	if err := tcp.WriteFrame(conn, raw); err != nil {
		s.removeConnState(conn)
	}
}

func (s *Server) handleAuthV2Frame(conn net.Conn, payload []byte, state *connState) {
	var auth secureproto.AuthFrame
	if err := json.Unmarshal(payload, &auth); err != nil {
		s.failConn(conn, "invalid auth_v2")
		return
	}
	if auth.Type != secureproto.TypeAuth || auth.ProtoVersion != "v2" {
		s.failConn(conn, "bad auth_v2 frame")
		return
	}
	if !secureproto.VerifyAuthProof(state.sessionKey, state.pendingTicket, auth.Proof) {
		s.failConn(conn, "auth_v2 proof mismatch")
		return
	}
	state.awaitAuthV2 = false
	state.pendingTicket = ""
	state.secureReady = true
	state.sessionID = s.newSessionID()
	s.bindSessionID(state.sessionID, state)
	ready := secureproto.BuildReady(state.sessionKey, state.sessionID)
	log.Printf("handshake_v2: sending ready sessionID=%s mac=%s", state.sessionID, ready.Mac)
	raw, _ := json.Marshal(ready)
	if err := tcp.WriteFrame(conn, raw); err != nil {
		s.removeConnState(conn)
	}
}

func (s *Server) newSessionID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func (s *Server) handleDataFrame(conn net.Conn, payload []byte, state *connState) {
	if state.protoVersion == "v2" {
		s.handleDataFrameV2(conn, payload, state)
		return
	}
	s.handleDataFrameV1(conn, payload, state)
}

func (s *Server) handleDataFrameV1(conn net.Conn, payload []byte, state *connState) {
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
	if s.tunnel != nil && isIPPacket(plain) {
		state.tunnelEnabled = true
		if err := s.tunnel.WritePacket(plain); err != nil {
			s.failConn(conn, "tunnel write failed")
		}
		return
	}
	reply := s.decorateReply(plain)
	if err := s.sendSecurePayload(state, reply); err != nil {
		s.removeConnState(conn)
	}
}

func (s *Server) handleDataFrameV2(conn net.Conn, payload []byte, state *connState) {
	frame, plain, err := secureproto.ParseDataFrameV2(state.sessionKey, payload)
	if err != nil {
		s.failConn(conn, "bad v2 data frame")
		return
	}
	if len(plain) > s.cfg.MaxPacketSize {
		s.failConn(conn, "payload too large")
		return
	}
	if s.tunnel != nil && frame.Channel == 1 && isIPPacket(plain) {
		state.tunnelEnabled = true
		if err := s.tunnel.WritePacket(plain); err != nil {
			s.failConn(conn, "tunnel write failed")
		}
		return
	}
	reply := s.decorateReply(plain)
	mode := secureproto.SelectObfsMode(frame.Sequence+1, len(reply))
	if err := s.sendSecurePayloadV2(state, frame.Channel, frame.Sequence+1, mode, reply); err != nil {
		s.removeConnState(conn)
	}
}

func (s *Server) sendSecurePayload(state *connState, payload []byte) error {
	enc, err := secureproto.Encrypt(state.sessionKey, payload)
	if err != nil {
		return err
	}
	resp, _ := json.Marshal(secureproto.DataFrame{
		Type:       secureproto.TypeData,
		Ciphertext: enc,
	})
	state.writeMu.Lock()
	defer state.writeMu.Unlock()
	return tcp.WriteFrame(state.conn, resp)
}

func (s *Server) sendSecurePayloadV2(state *connState, channel uint8, seq uint32, mode string, payload []byte) error {
	raw, err := secureproto.BuildDataFrameV2(state.sessionKey, channel, seq, mode, payload, 64)
	if err != nil {
		return err
	}
	state.writeMu.Lock()
	defer state.writeMu.Unlock()
	return tcp.WriteFrame(state.conn, raw)
}

func isIPPacket(packet []byte) bool {
	if len(packet) < 1 {
		return false
	}
	v := packet[0] >> 4
	return v == 4 || v == 6
}
