//go:build windows

package winclient

import (
	"errors"

	"golang.zx2c4.com/wintun"
	"golang.org/x/sys/windows"
)

type wireguardTunDevice struct {
	adapter *wintun.Adapter
	session wintun.Session
	iface   string
}

func openTunnelDevice(name string) (TunnelDevice, error) {
	adapter, err := wintun.CreateAdapter(name, "Wintun", nil)
	if err != nil {
		adapter, err = wintun.OpenAdapter(name)
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
		iface:   name,
	}, nil
}

func (w *wireguardTunDevice) ReadPacket() ([]byte, error) {
	for {
		packet, err := w.session.ReceivePacket()
		if err != nil {
			if !errors.Is(err, windows.ERROR_NO_MORE_ITEMS) {
				return nil, err
			}
			_, waitErr := windows.WaitForSingleObject(w.session.ReadWaitEvent(), windows.INFINITE)
			if waitErr != nil {
				return nil, waitErr
			}
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
	return w.iface, nil
}
