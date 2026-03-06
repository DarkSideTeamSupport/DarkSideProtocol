package winclient

import (
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

type TunnelDevice interface {
	ReadPacket() ([]byte, error)
	WritePacket([]byte) error
	Close() error
	Name() (string, error)
}

type defaultRouteInfo struct {
	NextHop string `json:"NextHop"`
	IfIndex int    `json:"ifIndex"`
}

func configureTunnelInterface(name string, cidr string, gateway string, setDefaultRoute bool, serverEndpoint string) error {
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
		if err := ensureServerBypassRoute(serverEndpoint); err != nil {
			return err
		}
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

func cleanupTunnelInterface(name string, gateway string, setDefaultRoute bool, serverEndpoint string) {
	if runtime.GOOS != "windows" {
		return
	}
	if !setDefaultRoute {
		return
	}
	_ = runNetsh("interface", "ipv4", "delete", "route", "0.0.0.0/0", fmt.Sprintf("interface=%s", name))
	_ = runNetsh("interface", "ipv4", "delete", "route", "0.0.0.0/0", fmt.Sprintf("interface=%s", name), fmt.Sprintf("nexthop=%s", gateway))
	_ = runNetsh("interface", "ip", "set", "dns", fmt.Sprintf("name=%s", name), "dhcp")
	_ = removeServerBypassRoute(serverEndpoint)
}

func runNetsh(args ...string) error {
	cmd := exec.Command("netsh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("netsh %v failed: %v (%s)", args, err, string(out))
	}
	return nil
}

func runRoute(args ...string) error {
	cmd := exec.Command("route", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("route %v failed: %v (%s)", args, err, string(out))
	}
	return nil
}

func ensureServerBypassRoute(serverEndpoint string) error {
	serverIP, err := resolveEndpointToIPv4(serverEndpoint)
	if err != nil {
		return nil
	}
	defRoute, err := getPrimaryDefaultRoute()
	if err != nil {
		return nil
	}
	if defRoute.NextHop == "" || defRoute.IfIndex <= 0 {
		return nil
	}
	_ = runRoute("DELETE", serverIP, "MASK", "255.255.255.255")
	if err := runRoute("ADD", serverIP, "MASK", "255.255.255.255", defRoute.NextHop, "METRIC", "3", "IF", strconv.Itoa(defRoute.IfIndex)); err != nil {
		// Keep tunnel startup resilient even when host-route already exists or route command differs.
		return nil
	}
	return nil
}

func removeServerBypassRoute(serverEndpoint string) error {
	serverIP, err := resolveEndpointToIPv4(serverEndpoint)
	if err != nil {
		return nil
	}
	_ = runRoute("DELETE", serverIP, "MASK", "255.255.255.255")
	return nil
}

func resolveEndpointToIPv4(endpoint string) (string, error) {
	if endpoint == "" {
		return "", fmt.Errorf("empty endpoint")
	}
	host := endpoint
	if h, _, err := net.SplitHostPort(endpoint); err == nil {
		host = h
	}
	if ip := net.ParseIP(host); ip != nil {
		v4 := ip.To4()
		if v4 == nil {
			return "", fmt.Errorf("non-ipv4 endpoint")
		}
		return v4.String(), nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return "", err
	}
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			return v4.String(), nil
		}
	}
	return "", fmt.Errorf("ipv4 address not found for endpoint")
}

func getPrimaryDefaultRoute() (defaultRouteInfo, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "$r=Get-NetRoute -AddressFamily IPv4 -DestinationPrefix '0.0.0.0/0' | Sort-Object -Property RouteMetric | Select-Object -First 1 NextHop,ifIndex; $r | ConvertTo-Json -Compress")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return defaultRouteInfo{}, err
	}
	var route defaultRouteInfo
	if err := json.Unmarshal(out, &route); err != nil {
		return defaultRouteInfo{}, err
	}
	return route, nil
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
