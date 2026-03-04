package obfs

import (
	"crypto/rand"
	"math/big"
	"time"
)

type Config struct {
	Enabled     bool
	MaxPadding  int
	MaxJitterMS int
}

func ApplyPadding(cfg Config, payload []byte) []byte {
	if !cfg.Enabled || cfg.MaxPadding <= 0 {
		return payload
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(cfg.MaxPadding+1)))
	if err != nil {
		return payload
	}
	pad := int(n.Int64())
	out := make([]byte, 0, len(payload)+pad)
	out = append(out, payload...)
	for i := 0; i < pad; i++ {
		out = append(out, 0)
	}
	return out
}

func SleepJitter(cfg Config) {
	if !cfg.Enabled || cfg.MaxJitterMS <= 0 {
		return
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(cfg.MaxJitterMS+1)))
	if err != nil {
		return
	}
	time.Sleep(time.Duration(n.Int64()) * time.Millisecond)
}
