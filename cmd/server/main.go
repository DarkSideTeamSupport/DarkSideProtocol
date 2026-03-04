package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"darksideprotocol/internal/config"
	"darksideprotocol/internal/server"
)

func main() {
	cfgPath := os.Getenv("DSP_SERVER_CONFIG")
	if cfgPath == "" {
		cfgPath = "configs/server.json"
	}

	cfg, err := config.LoadServerConfig(cfgPath)
	if err != nil {
		log.Fatalf("load server config: %v", err)
	}

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("create server: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("server starting: udp=%s tcp=%s", cfg.ListenUDP, cfg.ListenTCP)
	if err := srv.Run(ctx); err != nil {
		log.Fatalf("server stopped with error: %v", err)
	}
}
