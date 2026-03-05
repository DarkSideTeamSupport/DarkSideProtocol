package server

import (
	"context"
	"log"
	"net"
	"sync"

	"darksideprotocol/internal/config"
	"darksideprotocol/internal/obfs"
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
	lastSeen    time.Time
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
	errCh := make(chan error, 3)

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.reapIdleSessions(ctx)
	}()

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
				if len(payload) > s.cfg.MaxPacketSize {
					return
				}
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
