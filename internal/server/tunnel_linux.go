//go:build linux

package server

import (
	"fmt"
	"os/exec"
	"strings"

	"darksideprotocol/internal/config"
	"github.com/songgao/water"
)

type linuxTunnelDevice struct {
	iface *water.Interface
}

func newTunnelDevice(cfg config.ServerConfig) (tunnelDevice, error) {
	wcfg := water.Config{
		DeviceType: water.TUN,
	}
	wcfg.Name = cfg.TunnelName
	iface, err := water.New(wcfg)
	if err != nil {
		return nil, err
	}
	if err := configureLinuxTunnel(cfg, iface.Name()); err != nil {
		_ = iface.Close()
		return nil, err
	}
	return &linuxTunnelDevice{iface: iface}, nil
}

func (l *linuxTunnelDevice) ReadPacket() ([]byte, error) {
	buf := make([]byte, 65535)
	n, err := l.iface.Read(buf)
	if err != nil {
		return nil, err
	}
	out := make([]byte, n)
	copy(out, buf[:n])
	return out, nil
}

func (l *linuxTunnelDevice) WritePacket(packet []byte) error {
	_, err := l.iface.Write(packet)
	return err
}

func (l *linuxTunnelDevice) Close() error {
	return l.iface.Close()
}

func configureLinuxTunnel(cfg config.ServerConfig, name string) error {
	if err := runCmd("ip", "addr", "add", cfg.TunnelServerCIDR, "dev", name); err != nil && !strings.Contains(err.Error(), "File exists") {
		return err
	}
	if err := runCmd("ip", "link", "set", "dev", name, "up"); err != nil {
		return err
	}
	_ = runCmd("sysctl", "-w", "net.ipv4.ip_forward=1")
	upstream := cfg.UpstreamInterface
	if upstream == "" {
		upstream = "eth0"
	}
	_ = runCmd("iptables", "-t", "nat", "-C", "POSTROUTING", "-s", cfg.TunnelSubnet, "-o", upstream, "-j", "MASQUERADE")
	_ = runCmd("iptables", "-A", "FORWARD", "-i", name, "-j", "ACCEPT")
	_ = runCmd("iptables", "-A", "FORWARD", "-o", name, "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT")
	_ = runCmd("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", cfg.TunnelSubnet, "-o", upstream, "-j", "MASQUERADE")
	return nil
}

func runCmd(bin string, args ...string) error {
	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v failed: %v (%s)", bin, args, err, string(out))
	}
	return nil
}
