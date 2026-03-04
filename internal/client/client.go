package client

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"darksideprotocol/internal/config"
	"darksideprotocol/internal/obfs"
	"darksideprotocol/internal/policy"
	"darksideprotocol/internal/transport/tcp"
	"darksideprotocol/internal/transport/udp"
)

type Client struct {
	cfg     config.ClientConfig
	policy  *policy.Engine
	obfsCfg obfs.Config
}

func New(cfg config.ClientConfig) (*Client, error) {
	mode := policy.ModeUDP
	if strings.EqualFold(cfg.TransportMode, string(policy.ModeTCP)) {
		mode = policy.ModeTCP
	}

	return &Client{
		cfg:    cfg,
		policy: policy.New(mode),
		obfsCfg: obfs.Config{
			Enabled:     cfg.EnableObfs,
			MaxPadding:  24,
			MaxJitterMS: 10,
		},
	}, nil
}

func (c *Client) Run(ctx context.Context) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := c.pingOnce(); err != nil {
				log.Printf("ping failed mode=%s err=%v", c.policy.Current(), err)
				if c.policy.Current() == policy.ModeUDP {
					c.policy.ReportUDPFault()
				}
				continue
			}
			if c.policy.Current() == policy.ModeUDP {
				c.policy.ReportUDPHealthy()
			}
		}
	}
}

func (c *Client) pingOnce() error {
	msg := []byte("hello")
	msg = obfs.ApplyPadding(c.obfsCfg, msg)
	obfs.SleepJitter(c.obfsCfg)

	switch c.policy.Current() {
	case policy.ModeUDP:
		if c.cfg.ServerUDP == "" {
			return fmt.Errorf("server_udp is empty")
		}
		uc, err := udp.Dial(c.cfg.ServerUDP)
		if err != nil {
			return err
		}
		defer uc.Close()
		if err := uc.Send(msg); err != nil {
			return err
		}
		resp, err := uc.Receive(3 * time.Second)
		if err != nil {
			return err
		}
		log.Printf("udp response: %q", string(resp))
		return nil
	case policy.ModeTCP:
		if c.cfg.ServerTCP == "" {
			return fmt.Errorf("server_tcp is empty")
		}
		tc, err := tcp.Dial(c.cfg.ServerTCP)
		if err != nil {
			return err
		}
		defer tc.Close()
		if err := tc.Send(msg); err != nil {
			return err
		}
		resp, err := tc.Receive(3 * time.Second)
		if err != nil {
			return err
		}
		log.Printf("tcp response: %q", string(resp))
		return nil
	default:
		return fmt.Errorf("unknown mode: %s", c.policy.Current())
	}
}
