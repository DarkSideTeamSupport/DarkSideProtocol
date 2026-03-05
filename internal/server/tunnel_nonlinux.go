//go:build !linux

package server

import (
	"fmt"

	"darksideprotocol/internal/config"
)

func newTunnelDevice(_ config.ServerConfig) (tunnelDevice, error) {
	return nil, fmt.Errorf("tunnel mode requires linux server")
}
