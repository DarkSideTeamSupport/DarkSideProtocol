package winclient

import (
	"context"
	"fmt"
)

type App struct {
	cfg    Config
	client *SecureTCPClient
}

func New(cfg Config) (*App, error) {
	if cfg.ServerPublicKey == "" {
		return nil, fmt.Errorf("server_public_key is required")
	}
	keyStore := NewKeyStore(cfg.KeyFile)
	if err := keyStore.Ensure(&cfg); err != nil {
		return nil, err
	}
	return &App{
		cfg:    cfg,
		client: NewSecureTCPClient(cfg),
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	return a.client.Run(ctx)
}
