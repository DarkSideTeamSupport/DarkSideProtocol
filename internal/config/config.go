package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type ServerConfig struct {
	ListenUDP       string `json:"listen_udp"`
	ListenTCP       string `json:"listen_tcp"`
	PreSharedKey    string `json:"pre_shared_key"`
	ServerPrivateKey string `json:"server_private_key"`
	ServerPublicKey  string `json:"server_public_key"`
	MaxPacketSize   int    `json:"max_packet_size"`
	HandshakeSkewSec int   `json:"handshake_skew_sec"`
	SessionIdleSec   int   `json:"session_idle_sec"`
	EnableTunnel     bool   `json:"enable_tunnel"`
	TunnelName       string `json:"tunnel_name"`
	TunnelServerCIDR string `json:"tunnel_server_cidr"`
	TunnelSubnet     string `json:"tunnel_subnet"`
	UpstreamInterface string `json:"upstream_interface"`
	EnableObfs      bool   `json:"enable_obfs"`
	EnableUDP       bool   `json:"enable_udp"`
	EnableTCP       bool   `json:"enable_tcp"`
	PublicProbeMode bool   `json:"public_probe_mode"`
}

type ClientConfig struct {
	ServerAddress string `json:"server_address"`
	ServerUDP     string `json:"server_udp"`
	ServerTCP     string `json:"server_tcp"`
	PreSharedKey  string `json:"pre_shared_key"`
	ServerPublicKey string `json:"server_public_key"`
	ClientPrivateKey string `json:"client_private_key"`
	ClientPublicKey  string `json:"client_public_key"`
	KeyFile          string `json:"key_file"`
	TransportMode string `json:"transport_mode"`
	TunName       string `json:"tun_name"`
	TunCIDR       string `json:"tun_cidr"`
	TunGateway    string `json:"tun_gateway"`
	EnableTunnel  bool   `json:"enable_tunnel"`
	EnableObfs    bool   `json:"enable_obfs"`
}

type PanelConfig struct {
	ListenAddr      string `json:"listen_addr"`
	AdminUser       string `json:"admin_user"`
	AdminPassword   string `json:"admin_password"`
	StateFile       string `json:"state_file"`
	ServerLogPath   string `json:"server_log_path"`
	EnableServiceOp bool   `json:"enable_service_op"`
	ServiceName     string `json:"service_name"`
}

func LoadServerConfig(path string) (ServerConfig, error) {
	var cfg ServerConfig
	if err := loadJSON(path, &cfg); err != nil {
		return ServerConfig{}, err
	}
	if cfg.ListenUDP == "" {
		cfg.ListenUDP = ":18080"
	}
	if cfg.ListenTCP == "" {
		cfg.ListenTCP = ":18443"
	}
	if cfg.MaxPacketSize == 0 {
		cfg.MaxPacketSize = 1500
	}
	if cfg.HandshakeSkewSec <= 0 {
		cfg.HandshakeSkewSec = 120
	}
	if cfg.SessionIdleSec <= 0 {
		cfg.SessionIdleSec = 300
	}
	if cfg.TunnelName == "" {
		cfg.TunnelName = "dsp0"
	}
	if cfg.TunnelServerCIDR == "" {
		cfg.TunnelServerCIDR = "10.66.0.1/24"
	}
	if cfg.TunnelSubnet == "" {
		cfg.TunnelSubnet = "10.66.0.0/24"
	}
	if !cfg.EnableTunnel {
		// Default to tunnel mode for full VPN behavior.
		cfg.EnableTunnel = true
	}
	if !cfg.EnableUDP && !cfg.EnableTCP {
		cfg.EnableUDP = true
		cfg.EnableTCP = true
	}
	return cfg, nil
}

func LoadClientConfig(path string) (ClientConfig, error) {
	var cfg ClientConfig
	if err := loadJSON(path, &cfg); err != nil {
		return ClientConfig{}, err
	}
	if cfg.TransportMode == "" {
		cfg.TransportMode = "udp"
	}
	if cfg.TunName == "" {
		cfg.TunName = "dsp0"
	}
	if cfg.TunCIDR == "" {
		cfg.TunCIDR = "10.66.0.2/24"
	}
	if cfg.TunGateway == "" {
		cfg.TunGateway = "10.66.0.1"
	}
	if cfg.KeyFile == "" {
		cfg.KeyFile = "data/client-key.json"
	}
	return cfg, nil
}

func LoadPanelConfig(path string) (PanelConfig, error) {
	var cfg PanelConfig
	if err := loadJSON(path, &cfg); err != nil {
		return PanelConfig{}, err
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":2053"
	}
	if cfg.AdminUser == "" {
		cfg.AdminUser = "admin"
	}
	if cfg.AdminPassword == "" {
		cfg.AdminPassword = "admin123"
	}
	if cfg.StateFile == "" {
		cfg.StateFile = "data/panel-state.json"
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = "dsp-server.service"
	}
	return cfg, nil
}

func loadJSON(path string, out any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}
