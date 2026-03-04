package panel

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

type sessionInfo struct {
	User      string
	ExpiresAt time.Time
}

type authManager struct {
	user         string
	passwordHash string
	sessions     map[string]sessionInfo
	mu           sync.RWMutex
}

func newAuthManager(user, password string) *authManager {
	sum := sha256.Sum256([]byte(password))
	return &authManager{
		user:         user,
		passwordHash: hex.EncodeToString(sum[:]),
		sessions:     make(map[string]sessionInfo),
	}
}

func (a *authManager) Login(user, password string) (string, bool) {
	sum := sha256.Sum256([]byte(password))
	if user != a.user || hex.EncodeToString(sum[:]) != a.passwordHash {
		return "", false
	}
	token := make([]byte, 24)
	if _, err := rand.Read(token); err != nil {
		return "", false
	}
	id := hex.EncodeToString(token)
	a.mu.Lock()
	a.sessions[id] = sessionInfo{
		User:      user,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	a.mu.Unlock()
	return id, true
}

func (a *authManager) Validate(token string) bool {
	a.mu.RLock()
	s, ok := a.sessions[token]
	a.mu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().After(s.ExpiresAt) {
		a.mu.Lock()
		delete(a.sessions, token)
		a.mu.Unlock()
		return false
	}
	return true
}

func (a *authManager) Logout(token string) {
	a.mu.Lock()
	delete(a.sessions, token)
	a.mu.Unlock()
}
