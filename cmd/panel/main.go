package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"darksideprotocol/internal/config"
	"darksideprotocol/internal/panel"
)

func main() {
	cfgPath := os.Getenv("DSP_PANEL_CONFIG")
	if cfgPath == "" {
		cfgPath = "configs/panel.json"
	}

	cfg, err := config.LoadPanelConfig(cfgPath)
	if err != nil {
		log.Fatalf("load panel config: %v", err)
	}

	srv, err := panel.New(cfg)
	if err != nil {
		log.Fatalf("create panel: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("panel starting at %s", cfg.ListenAddr)
	if err := srv.Run(ctx); err != nil {
		log.Fatalf("panel stopped with error: %v", err)
	}
}
