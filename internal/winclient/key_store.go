package winclient

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"darksideprotocol/internal/secureproto"
)

type keyFilePayload struct {
	ClientPrivateKey string `json:"client_private_key"`
	ClientPublicKey  string `json:"client_public_key"`
}

type KeyStore struct {
	path string
}

func NewKeyStore(path string) *KeyStore {
	return &KeyStore{path: path}
}

func (k *KeyStore) Ensure(cfg *Config) error {
	if cfg.ClientPrivateKey != "" && cfg.ClientPublicKey != "" {
		return nil
	}
	if err := k.loadIntoConfig(cfg); err == nil {
		return nil
	}
	pair, err := secureproto.GenerateKeyPair()
	if err != nil {
		return err
	}
	cfg.ClientPrivateKey = pair.PrivateKey
	cfg.ClientPublicKey = pair.PublicKey
	return k.savePair(pair)
}

func (k *KeyStore) loadIntoConfig(cfg *Config) error {
	b, err := os.ReadFile(k.path)
	if err != nil {
		return err
	}
	var data keyFilePayload
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	if data.ClientPrivateKey == "" || data.ClientPublicKey == "" {
		return fmt.Errorf("key file is incomplete")
	}
	cfg.ClientPrivateKey = data.ClientPrivateKey
	cfg.ClientPublicKey = data.ClientPublicKey
	return nil
}

func (k *KeyStore) savePair(pair secureproto.KeyPair) error {
	if err := os.MkdirAll(filepath.Dir(k.path), 0o755); err != nil {
		return err
	}
	data := keyFilePayload{
		ClientPrivateKey: pair.PrivateKey,
		ClientPublicKey:  pair.PublicKey,
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(k.path, b, 0o600)
}
