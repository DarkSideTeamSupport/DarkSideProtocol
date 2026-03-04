package winclient

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	ServerTCP        string `json:"server_tcp"`
	ServerPublicKey  string `json:"server_public_key"`
	ClientPrivateKey string `json:"client_private_key"`
	ClientPublicKey  string `json:"client_public_key"`
	KeyFile          string `json:"key_file"`
	ProfileName      string `json:"profile_name"`
	DeviceName       string `json:"device_name"`
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
	if cfg.ServerTCP == "" {
		cfg.ServerTCP = "127.0.0.1:18443"
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
