package server

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"log"
	"net"
	"sync"

	"darksideprotocol/internal/config"
	"darksideprotocol/internal/obfs"
	"darksideprotocol/internal/secureproto"
	"darksideprotocol/internal/transport/tcp"
	"darksideprotocol/internal/transport/udp"
)

type Server struct {
	cfg     config.ServerConfig
	obfsCfg obfs.Config
	connMu  sync.Mutex
	conns   map[net.Conn]*connState
}

type connState struct {
	secureReady bool
	sessionKey  []byte
}

func New(cfg config.ServerConfig) (*Server, error) {
	return &Server{
		cfg: cfg,
		obfsCfg: obfs.Config{
			Enabled:     cfg.EnableObfs,
			MaxPadding:  32,
			MaxJitterMS: 15,
		},
		conns: make(map[net.Conn]*connState),
	}, nil
}

func (s *Server) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	if s.cfg.EnableUDP {
		wg.Add(1)
		go func() {
			defer wg.Done()
			udpSrv, err := udp.Listen(s.cfg.ListenUDP)
			if err != nil {
				errCh <- err
				return
			}
			log.Printf("udp listener started on %s", s.cfg.ListenUDP)
			err = udpSrv.Serve(ctx, func(addr *net.UDPAddr, payload []byte) {
				_ = udpSrv.WriteTo(addr, s.decorateReply(payload))
			})
			if err != nil {
				errCh <- err
			}
		}()
	}

	if s.cfg.EnableTCP {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tcpSrv, err := tcp.Listen(s.cfg.ListenTCP)
			if err != nil {
				errCh <- err
				return
			}
			log.Printf("tcp listener started on %s", s.cfg.ListenTCP)
			err = tcpSrv.Serve(ctx, func(conn net.Conn, payload []byte) {
				s.handleTCPPayload(conn, payload)
			})
			if err != nil {
				errCh <- err
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		<-done
		return nil
	case err := <-errCh:
		return err
	}
}

func (s *Server) decorateReply(payload []byte) []byte {
	out := append([]byte("dsp-ok:"), payload...)
	out = obfs.ApplyPadding(s.obfsCfg, out)
	obfs.SleepJitter(s.obfsCfg)
	return out
}

func (s *Server) handleTCPPayload(conn net.Conn, payload []byte) {
	if s.cfg.ServerPrivateKey == "" {
		_ = tcp.WriteFrame(conn, s.decorateReply(payload))
		return
	}

	state := s.getConnState(conn)
	if !state.secureReady {
		s.handleHelloFrame(conn, payload, state)
		return
	}
	s.handleDataFrame(conn, payload, state)
}

func (s *Server) handleHelloFrame(conn net.Conn, payload []byte, state *connState) {
	hello, clientNonce, err := secureproto.ParseHello(payload)
	if err != nil {
		_ = tcp.WriteFrame(conn, []byte(`{"type":"error","message":"invalid hello"}`))
		return
	}
	shared, err := secureproto.SharedSecret(s.cfg.ServerPrivateKey, hello.ClientPublicKey)
	if err != nil {
		_ = tcp.WriteFrame(conn, []byte(`{"type":"error","message":"handshake failed"}`))
		return
	}

	serverNonce := make([]byte, 16)
	if _, err := rand.Read(serverNonce); err != nil {
		_ = tcp.WriteFrame(conn, []byte(`{"type":"error","message":"nonce failed"}`))
		return
	}
	state.sessionKey = secureproto.DeriveSessionKey(shared, clientNonce, serverNonce)
	state.secureReady = true
	ack := secureproto.BuildAck(state.sessionKey, serverNonce)
	raw, _ := json.Marshal(ack)
	_ = tcp.WriteFrame(conn, raw)
}

func (s *Server) handleDataFrame(conn net.Conn, payload []byte, state *connState) {
	var req secureproto.DataFrame
	if err := json.Unmarshal(payload, &req); err != nil || req.Type != secureproto.TypeData {
		_ = tcp.WriteFrame(conn, []byte(`{"type":"error","message":"bad secure frame"}`))
		return
	}
	plain, err := secureproto.Decrypt(state.sessionKey, req.Ciphertext)
	if err != nil {
		_ = tcp.WriteFrame(conn, []byte(`{"type":"error","message":"decrypt failed"}`))
		return
	}
	reply := s.decorateReply(plain)
	enc, err := secureproto.Encrypt(state.sessionKey, reply)
	if err != nil {
		_ = tcp.WriteFrame(conn, []byte(`{"type":"error","message":"encrypt failed"}`))
		return
	}
	resp, _ := json.Marshal(secureproto.DataFrame{
		Type:       secureproto.TypeData,
		Ciphertext: enc,
	})
	_ = tcp.WriteFrame(conn, resp)
}

func (s *Server) getConnState(conn net.Conn) *connState {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	st, ok := s.conns[conn]
	if ok {
		return st
	}
	st = &connState{}
	s.conns[conn] = st
	return st
}
