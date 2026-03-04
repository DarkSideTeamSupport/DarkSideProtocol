package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"darksideprotocol/internal/winclient"
)

func main() {
	cfgPath := os.Getenv("DSP_WINCLIENT_CONFIG")
	if cfgPath == "" {
		cfgPath = "configs/winclient.json"
	}
	cfg, err := winclient.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("load win client config: %v", err)
	}

	app, err := winclient.New(cfg)
	if err != nil {
		log.Fatalf("init win client: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx); err != nil {
		log.Fatalf("run win client: %v", err)
	}
}
