package tun

import "fmt"

type Device interface {
	ReadPacket() ([]byte, error)
	WritePacket([]byte) error
	Close() error
}

func Open(name string, cidr string) (Device, error) {
	return nil, fmt.Errorf("TUN is not implemented yet (name=%s cidr=%s)", name, cidr)
}
