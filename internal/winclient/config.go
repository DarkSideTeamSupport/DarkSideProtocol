package winclient

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	ServerTCP        string `json:"server_tcp"`
	ServerUDP        string `json:"server_udp"`
	ProtocolVersion  string `json:"protocol_version"`
	EnableMultiTransport bool `json:"enable_multi_transport"`
	PreSharedKey     string `json:"pre_shared_key"`
	ServerPublicKey  string `json:"server_public_key"`
	ClientPrivateKey string `json:"client_private_key"`
	ClientPublicKey  string `json:"client_public_key"`
	KeyFile          string `json:"key_file"`
	ProfileName      string `json:"profile_name"`
	DeviceName       string `json:"device_name"`
	EnableTunnel     bool   `json:"enable_tunnel"`
	TunName          string `json:"tun_name"`
	TunCIDR          string `json:"tun_cidr"`
	TunGateway       string `json:"tun_gateway"`
	TunSetDefaultRoute bool `json:"tun_set_default_route"`
	TunProbeOnly       bool `json:"tun_probe_only"`
	ReconnectSec     int    `json:"reconnect_sec"`
	PingIntervalSec  int    `json:"ping_interval_sec"`
	HandshakeTimeout int    `json:"handshake_timeout_sec"`
}

func LoadConfig(path string) (Config, error) {
	var cfg Config
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(b, &raw)
	if cfg.ServerTCP == "" {
		cfg.ServerTCP = "127.0.0.1:18443"
	}
	if cfg.ProtocolVersion == "" {
		cfg.ProtocolVersion = "v2"
	}
	if cfg.ServerUDP == "" {
		cfg.ServerUDP = "127.0.0.1:18080"
	}
	if _, ok := raw["enable_multi_transport"]; !ok {
		cfg.EnableMultiTransport = true
	}
	if cfg.KeyFile == "" {
		cfg.KeyFile = "data/windows-client-key.json"
	}
	if cfg.ProfileName == "" {
		cfg.ProfileName = "default"
	}
	if cfg.DeviceName == "" {
		cfg.DeviceName = "win-client"
	}
	if !cfg.EnableTunnel {
		cfg.EnableTunnel = true
	}
	if cfg.TunName == "" {
		cfg.TunName = "DarkSideTunnel"
	}
	if cfg.TunCIDR == "" {
		cfg.TunCIDR = "10.66.0.2/24"
	}
	if cfg.TunGateway == "" {
		cfg.TunGateway = "10.66.0.1"
	}
	// Safe defaults when fields are absent in config:
	// - do not switch default route
	// - run probe mode only (without dataplane loops)
	if _, ok := raw["tun_set_default_route"]; !ok {
		cfg.TunSetDefaultRoute = false
	}
	if _, ok := raw["tun_probe_only"]; !ok {
		cfg.TunProbeOnly = true
	}
	if cfg.ReconnectSec <= 0 {
		cfg.ReconnectSec = 3
	}
	if cfg.PingIntervalSec <= 0 {
		cfg.PingIntervalSec = 3
	}
	if cfg.HandshakeTimeout <= 0 {
		cfg.HandshakeTimeout = 5
	}
	return cfg, nil
}
