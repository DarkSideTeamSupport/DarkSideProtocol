package server

import (
	"context"
	"time"

	"darksideprotocol/internal/secureproto"
)

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
		if state.protoVersion == "v2" {
			if state.udpAddr != nil && s.getUDPSrv() != nil && state.sessionID != "" {
				mode := secureproto.SelectObfsMode(uint32(time.Now().UnixNano()), len(packet))
				raw, err := secureproto.BuildDatagramFrameV2(state.sessionKey, state.sessionID, 1, uint32(time.Now().UnixNano()), mode, packet, 48)
				if err == nil {
					if err := s.getUDPSrv().WriteTo(state.udpAddr, raw); err == nil {
						continue
					}
				}
			}
			_ = s.sendSecurePayloadV2(state, 1, uint32(time.Now().UnixNano()), "obfs-plane", packet)
			continue
		}
		_ = s.sendSecurePayload(state, packet)
	}
}
