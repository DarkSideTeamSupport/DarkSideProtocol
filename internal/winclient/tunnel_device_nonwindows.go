//go:build !windows

package winclient

import "fmt"

func openTunnelDevice(_ string) (TunnelDevice, error) {
	return nil, fmt.Errorf("tunnel device is supported only on windows")
}
