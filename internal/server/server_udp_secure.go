package server

import (
	"net"
	"time"

	"darksideprotocol/internal/secureproto"
)

func (s *Server) handleUDPPayload(udpAddr *net.UDPAddr, payload []byte) {
	if len(payload) == 0 {
		return
	}
	udpSrv := s.getUDPSrv()
	if udpSrv == nil {
		return
	}
	if s.cfg.ServerPrivateKey == "" {
		if len(payload) <= s.cfg.MaxPacketSize {
			_ = udpSrv.WriteTo(udpAddr, s.decorateReply(payload))
		}
		return
	}
	if len(payload) > s.cfg.MaxPacketSize*8 {
		return
	}
	sid, err := secureproto.ParseDatagramSessionID(payload)
	if err != nil {
		return
	}
	state := s.findStateBySessionID(sid)
	if state == nil || !state.secureReady || state.protoVersion != "v2" {
		return
	}
	state.lastSeen = time.Now()
	state.udpAddr = udpAddr
	frame, plain, err := secureproto.ParseDatagramFrameV2(state.sessionKey, payload)
	if err != nil || len(plain) > s.cfg.MaxPacketSize {
		return
	}
	if frame.Mode == "bind" {
		reply, err := secureproto.BuildDatagramFrameV2(state.sessionKey, state.sessionID, 2, frame.Sequence+1, "obfs-plane", []byte("udp-bound"), 24)
		if err == nil {
			_ = udpSrv.WriteTo(udpAddr, reply)
		}
		return
	}
	if s.tunnel != nil && frame.Channel == 1 && isIPPacket(plain) {
		state.tunnelEnabled = true
		_ = s.tunnel.WritePacket(plain)
		return
	}
	reply := s.decorateReply(plain)
	mode := secureproto.SelectObfsMode(frame.Sequence+1, len(reply))
	resp, err := secureproto.BuildDatagramFrameV2(state.sessionKey, state.sessionID, frame.Channel, frame.Sequence+1, mode, reply, 64)
	if err != nil {
		return
	}
	_ = udpSrv.WriteTo(udpAddr, resp)
}
