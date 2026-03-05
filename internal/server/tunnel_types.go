package server

import "context"

type tunnelDevice interface {
	ReadPacket() ([]byte, error)
	WritePacket([]byte) error
	Close() error
}

func (s *Server) relayTunnelToClient(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		packet, err := s.tunnel.ReadPacket()
		if err != nil {
			continue
		}
		if len(packet) == 0 {
			continue
		}
		state := s.findSecureConnState()
		if state == nil {
			continue
		}
		_ = s.sendSecurePayload(state, packet)
	}
}
