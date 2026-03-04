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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("client starting: server=%s transport=%s", cfg.ServerAddress, cfg.TransportMode)
	if err := cl.Run(ctx); err != nil {
		log.Fatalf("client stopped with error: %v", err)
	}
}
