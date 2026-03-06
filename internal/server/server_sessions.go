package server

import (
	"context"
	"net"
	"time"

	"darksideprotocol/internal/transport/tcp"
)

func (s *Server) getConnState(conn net.Conn) *connState {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	st, ok := s.conns[conn]
	if ok {
		return st
	}
	st = &connState{lastSeen: time.Now()}
	st.conn = conn
	s.conns[conn] = st
	return st
}

func (s *Server) removeConnState(conn net.Conn) {
	s.connMu.Lock()
	st := s.conns[conn]
	delete(s.conns, conn)
	s.connMu.Unlock()
	if st != nil && st.sessionID != "" {
		s.unbindSessionID(st.sessionID)
	}
}

func (s *Server) failConn(conn net.Conn, message string) {
	_ = tcp.WriteFrame(conn, []byte(`{"type":"error","message":"`+message+`"}`))
	_ = conn.Close()
	s.removeConnState(conn)
}

func (s *Server) reapIdleSessions(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.dropIdleConnStates()
		}
	}
}

func (s *Server) dropIdleConnStates() {
	if s.cfg.SessionIdleSec <= 0 {
		return
	}
	cutoff := time.Now().Add(-time.Duration(s.cfg.SessionIdleSec) * time.Second)
	var (
		toClose []net.Conn
		sids    []string
	)

	s.connMu.Lock()
	for c, st := range s.conns {
		if st.lastSeen.Before(cutoff) {
			toClose = append(toClose, c)
			if st.sessionID != "" {
				sids = append(sids, st.sessionID)
			}
			delete(s.conns, c)
		}
	}
	s.connMu.Unlock()

	for _, sid := range sids {
		s.unbindSessionID(sid)
	}

	for _, c := range toClose {
		_ = c.Close()
	}
}

func (s *Server) bindSessionID(sessionID string, state *connState) {
	if sessionID == "" || state == nil {
		return
	}
	s.sidMu.Lock()
	defer s.sidMu.Unlock()
	s.bySID[sessionID] = state
}

func (s *Server) unbindSessionID(sessionID string) {
	if sessionID == "" {
		return
	}
	s.sidMu.Lock()
	defer s.sidMu.Unlock()
	delete(s.bySID, sessionID)
}

func (s *Server) findStateBySessionID(sessionID string) *connState {
	if sessionID == "" {
		return nil
	}
	s.sidMu.RLock()
	defer s.sidMu.RUnlock()
	return s.bySID[sessionID]
}

func (s *Server) findSecureConnState() *connState {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	for _, st := range s.conns {
		if st.secureReady && st.tunnelEnabled && st.conn != nil {
			return st
		}
	}
	return nil
}
