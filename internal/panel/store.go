package panel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	path  string
	mu    sync.RWMutex
	state State
}

func NewStore(path string) (*Store, error) {
	s := &Store{
		path: path,
		state: State{
			Inbounds: make([]Inbound, 0),
			Clients:  make([]Client, 0),
			Settings: DefaultSettings(),
		},
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return s.saveLocked()
		}
		return fmt.Errorf("read state: %w", err)
	}
	if len(b) == 0 {
		return nil
	}
	if err := json.Unmarshal(b, &s.state); err != nil {
		return fmt.Errorf("unmarshal state: %w", err)
	}
	s.applyDefaultsLocked()
	return nil
}

func (s *Store) applyDefaultsLocked() {
	if s.state.Inbounds == nil {
		s.state.Inbounds = make([]Inbound, 0)
	}
	if s.state.Clients == nil {
		s.state.Clients = make([]Client, 0)
	}

	def := DefaultSettings()
	if s.state.Settings.Panel.SiteTitle == "" {
		s.state.Settings.Panel.SiteTitle = def.Panel.SiteTitle
	}
	if s.state.Settings.Panel.DefaultLang == "" {
		s.state.Settings.Panel.DefaultLang = def.Panel.DefaultLang
	}
	if s.state.Settings.Panel.SessionHours == 0 {
		s.state.Settings.Panel.SessionHours = def.Panel.SessionHours
	}
	if s.state.Settings.Panel.Timezone == "" {
		s.state.Settings.Panel.Timezone = def.Panel.Timezone
	}
	if s.state.Settings.Transport.DefaultInboundTransport == "" {
		s.state.Settings.Transport.DefaultInboundTransport = def.Transport.DefaultInboundTransport
	}
	if s.state.Settings.Transport.DefaultPortTCP == 0 {
		s.state.Settings.Transport.DefaultPortTCP = def.Transport.DefaultPortTCP
	}
	if s.state.Settings.Transport.DefaultPortUDP == 0 {
		s.state.Settings.Transport.DefaultPortUDP = def.Transport.DefaultPortUDP
	}
	if !s.state.Settings.Transport.EnableTCP && !s.state.Settings.Transport.EnableUDP {
		s.state.Settings.Transport.EnableTCP = def.Transport.EnableTCP
		s.state.Settings.Transport.EnableUDP = def.Transport.EnableUDP
	}
	if s.state.Settings.Obfs.MaxPadding == 0 {
		s.state.Settings.Obfs.MaxPadding = def.Obfs.MaxPadding
	}
	if s.state.Settings.Obfs.MaxJitterMS == 0 {
		s.state.Settings.Obfs.MaxJitterMS = def.Obfs.MaxJitterMS
	}
	if s.state.Settings.Security.MaxIPsPerClient == 0 {
		s.state.Settings.Security.MaxIPsPerClient = def.Security.MaxIPsPerClient
	}
	if s.state.Settings.Security.AllowedCIDR == "" {
		s.state.Settings.Security.AllowedCIDR = def.Security.AllowedCIDR
	}
	if s.state.Settings.Traffic.DefaultClientGB == 0 {
		s.state.Settings.Traffic.DefaultClientGB = def.Traffic.DefaultClientGB
	}
	if s.state.Settings.Traffic.ResetEveryDays == 0 {
		s.state.Settings.Traffic.ResetEveryDays = def.Traffic.ResetEveryDays
	}
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("mkdir state dir: %w", err)
	}
	b, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	if err := os.WriteFile(s.path, b, 0o644); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	return nil
}

func (s *Store) Snapshot() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	inbounds := append([]Inbound{}, s.state.Inbounds...)
	clients := append([]Client{}, s.state.Clients...)
	cp := State{
		Inbounds: inbounds,
		Clients:  clients,
		Settings: s.state.Settings,
	}
	return cp
}

func (s *Store) UpsertInbound(in Inbound) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Inbounds {
		if s.state.Inbounds[i].ID == in.ID {
			s.state.Inbounds[i] = in
			return s.saveLocked()
		}
	}
	s.state.Inbounds = append(s.state.Inbounds, in)
	return s.saveLocked()
}

func (s *Store) DeleteInbound(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dst := s.state.Inbounds[:0]
	for _, v := range s.state.Inbounds {
		if v.ID != id {
			dst = append(dst, v)
		}
	}
	s.state.Inbounds = dst
	return s.saveLocked()
}

func (s *Store) UpsertClient(cl Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Clients {
		if s.state.Clients[i].ID == cl.ID {
			s.state.Clients[i] = cl
			return s.saveLocked()
		}
	}
	s.state.Clients = append(s.state.Clients, cl)
	return s.saveLocked()
}

func (s *Store) DeleteClient(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dst := s.state.Clients[:0]
	for _, v := range s.state.Clients {
		if v.ID != id {
			dst = append(dst, v)
		}
	}
	s.state.Clients = dst
	return s.saveLocked()
}

func (s *Store) UpdateSettings(settings Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Settings = settings
	s.applyDefaultsLocked()
	return s.saveLocked()
}
