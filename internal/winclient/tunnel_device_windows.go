//go:build windows

package winclient

import (
	"fmt"
	"time"

	"golang.zx2c4.com/wintun"
)

type wireguardTunDevice struct {
	adapter *wintun.Adapter
	session wintun.Session
}

func openTunnelDevice(name string) (TunnelDevice, error) {
	pool, err := wintun.MakePool("DarkSidePool")
	if err != nil {
		return nil, err
	}
	adapter, _, err := pool.CreateAdapter(name, nil)
	if err != nil {
		adapter, err = pool.OpenAdapter(name)
		if err != nil {
			return nil, err
		}
	}
	session, err := adapter.StartSession(0x800000)
	if err != nil {
		adapter.Close()
		return nil, err
	}
	return &wireguardTunDevice{
		adapter: adapter,
		session: session,
	}, nil
}

func (w *wireguardTunDevice) ReadPacket() ([]byte, error) {
	for {
		packet, err := w.session.ReceivePacket()
		if err != nil {
			<-w.session.ReadWaitEvent()
			time.Sleep(2 * time.Millisecond)
			continue
		}
		out := make([]byte, len(packet))
		copy(out, packet)
		w.session.ReleaseReceivePacket(packet)
		if len(out) == 0 {
			continue
		}
		return out, nil
	}
}

func (w *wireguardTunDevice) WritePacket(packet []byte) error {
	sendPacket, err := w.session.AllocateSendPacket(len(packet))
	if err != nil {
		return err
	}
	copy(sendPacket, packet)
	w.session.SendPacket(sendPacket)
	return nil
}

func (w *wireguardTunDevice) Close() error {
	w.session.End()
	w.adapter.Close()
	return nil
}

func (w *wireguardTunDevice) Name() (string, error) {
	name, err := w.adapter.Name()
	if err != nil {
		return "", fmt.Errorf("read adapter name: %w", err)
	}
	return name, nil
}
