package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"darksideprotocol/internal/client"
	"darksideprotocol/internal/config"
)

func main() {
	cfgPath := os.Getenv("DSP_CLIENT_CONFIG")
	if cfgPath == "" {
		cfgPath = "configs/client.json"
	}

	cfg, err := config.LoadClientConfig(cfgPath)
	if err != nil {
		log.Fatalf("load client config: %v", err)
	}

	cl, err := client.New(cfg)
	if err != nil {
		log.Fatalf("create client: %v", err)
	}
	if err := cl.Validate(); err != nil {
		log.Fatalf("invalid client config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("client starting: proto=%s tcp=%s udp=%s transport=%s", cfg.ProtocolVersion, cfg.ServerTCP, cfg.ServerUDP, cfg.TransportMode)
	if err := cl.Run(ctx); err != nil {
		log.Fatalf("client stopped with error: %v", err)
	}
}
