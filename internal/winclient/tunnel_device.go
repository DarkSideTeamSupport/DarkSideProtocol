package winclient

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

type TunnelDevice interface {
	ReadPacket() ([]byte, error)
	WritePacket([]byte) error
	Close() error
	Name() (string, error)
}

func configureTunnelInterface(name string, cidr string, gateway string, setDefaultRoute bool) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	ip, mask, err := cidrToMask(cidr)
	if err != nil {
		return err
	}
	// In safe/probe mode we must not inject default gateway at all,
	// otherwise Windows can auto-create 0.0.0.0/0 route through tunnel.
	gw := "none"
	if setDefaultRoute {
		gw = gateway
	}
	if err := runNetsh("interface", "ip", "set", "address", fmt.Sprintf("name=%s", name), "static", ip, mask, gw); err != nil {
		return err
	}
	if setDefaultRoute {
		_ = runNetsh("interface", "ipv4", "delete", "route", "0.0.0.0/0", fmt.Sprintf("interface=%s", name))
		if err := runNetsh("interface", "ipv4", "add", "route", "0.0.0.0/0", fmt.Sprintf("interface=%s", name), fmt.Sprintf("nexthop=%s", gateway), "metric=5", "store=active"); err != nil {
			return err
		}
		_ = runNetsh("interface", "ip", "set", "dns", fmt.Sprintf("name=%s", name), "static", "1.1.1.1")
	} else {
		_ = runNetsh("interface", "ipv4", "delete", "route", "0.0.0.0/0", fmt.Sprintf("interface=%s", name))
		_ = runNetsh("interface", "ipv4", "delete", "route", "0.0.0.0/0", fmt.Sprintf("interface=%s", name), fmt.Sprintf("nexthop=%s", gateway))
		_ = runNetsh("interface", "ip", "set", "dns", fmt.Sprintf("name=%s", name), "dhcp")
	}
	return nil
}

func runNetsh(args ...string) error {
	cmd := exec.Command("netsh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("netsh %v failed: %v (%s)", args, err, string(out))
	}
	return nil
}

func cidrToMask(cidr string) (string, string, error) {
	parts := strings.Split(cidr, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid cidr: %s", cidr)
	}
	ip := parts[0]
	switch parts[1] {
	case "24":
		return ip, "255.255.255.0", nil
	case "16":
		return ip, "255.255.0.0", nil
	default:
		return "", "", fmt.Errorf("unsupported mask /%s", parts[1])
	}
}
