package panel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"darksideprotocol/internal/config"
)

type Server struct {
	cfg      config.PanelConfig
	store    *Store
	auth     *authManager
	started  time.Time
	reqCount atomic.Uint64
}

func New(cfg config.PanelConfig) (*Server, error) {
	st, err := NewStore(cfg.StateFile)
	if err != nil {
		return nil, err
	}
	return &Server{
		cfg:     cfg,
		store:   st,
		auth:    newAuthManager(cfg.AdminUser, cfg.AdminPassword),
		started: time.Now(),
	}, nil
}

func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleSPA)
	mux.HandleFunc("/api/session", s.handleSession)
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/logout", s.authOnly(s.handleLogout))
	mux.HandleFunc("/api/me", s.authOnly(s.handleMe))
	mux.HandleFunc("/api/stats", s.authOnly(s.handleStats))
	mux.HandleFunc("/api/inbounds", s.authOnly(s.handleInbounds))
	mux.HandleFunc("/api/inbounds/", s.authOnly(s.handleInboundByID))
	mux.HandleFunc("/api/clients", s.authOnly(s.handleClients))
	mux.HandleFunc("/api/clients/", s.authOnly(s.handleClientByID))
	mux.HandleFunc("/api/settings", s.authOnly(s.handleSettings))
	mux.HandleFunc("/api/settings/reset", s.authOnly(s.handleSettingsReset))
	mux.HandleFunc("/api/service/", s.authOnly(s.handleService))
	mux.HandleFunc("/api/logs", s.authOnly(s.handleLogs))

	srv := &http.Server{
		Addr:              s.cfg.ListenAddr,
		Handler:           s.withCounter(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("panel listening on %s", s.cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func (s *Server) withCounter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.reqCount.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	frontendRoot, ok := findFrontendRoot()
	if !ok {
		http.Error(w, "frontend build not found", http.StatusServiceUnavailable)
		return
	}
	cleanPath := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	if cleanPath == "." || cleanPath == "" {
		http.ServeFile(w, r, filepath.Join(frontendRoot, "index.html"))
		return
	}
	target := filepath.Join(frontendRoot, cleanPath)
	if st, err := os.Stat(target); err == nil && !st.IsDir() {
		http.ServeFile(w, r, target)
		return
	}
	http.ServeFile(w, r, filepath.Join(frontendRoot, "index.html"))
}

func findFrontendRoot() (string, bool) {
	candidates := []string{
		"web-ui/dist",
		"web",
	}
	for _, root := range candidates {
		if st, err := os.Stat(filepath.Join(root, "index.html")); err == nil && !st.IsDir() {
			return root, true
		}
	}
	return "", false
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	token, ok := s.auth.Login(req.Username, req.Password)
	if !ok {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "dsp_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token := s.readSessionToken(r)
	authorized := token != "" && s.auth.Validate(token)
	writeJSON(w, map[string]bool{"authorized": authorized})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token := s.readSessionToken(r)
	if token != "" {
		s.auth.Logout(token)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "dsp_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]string{"user": s.cfg.AdminUser})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	st := s.store.Snapshot()
	stats := Stats{
		UptimeSeconds: int64(time.Since(s.started).Seconds()),
		GoRoutines:    runtime.NumGoroutine(),
		MemoryAlloc:   mem.Alloc,
		Inbounds:      len(st.Inbounds),
		Clients:       len(st.Clients),
	}
	writeJSON(w, map[string]any{
		"stats":         stats,
		"request_count": s.reqCount.Load(),
	})
}

func (s *Server) handleInbounds(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.store.Snapshot().Inbounds)
	case http.MethodPost:
		var req Inbound
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if req.Name == "" || req.Listen == "" || req.Transport == "" {
			http.Error(w, "name, listen, transport are required", http.StatusBadRequest)
			return
		}
		if req.ID == "" {
			req.ID = newID("inb")
			req.CreatedAt = time.Now().UTC()
		}
		if err := s.store.UpsertInbound(req); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, req)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleInboundByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/inbounds/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodPut:
		var req Inbound
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		req.ID = id
		if req.Name == "" || req.Listen == "" || req.Transport == "" {
			http.Error(w, "name, listen, transport are required", http.StatusBadRequest)
			return
		}
		if req.CreatedAt.IsZero() {
			req.CreatedAt = time.Now().UTC()
		}
		if err := s.store.UpsertInbound(req); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, req)
	case http.MethodDelete:
		if err := s.store.DeleteInbound(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "deleted"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleClients(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.store.Snapshot().Clients)
	case http.MethodPost:
		var req struct {
			ID        string `json:"id"`
			Email     string `json:"email"`
			InboundID string `json:"inbound_id"`
			ExpiresAt string `json:"expires_at"`
			Enabled   bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if req.Email == "" || req.InboundID == "" {
			http.Error(w, "email and inbound_id are required", http.StatusBadRequest)
			return
		}
		expiresAt, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			http.Error(w, "expires_at must be RFC3339", http.StatusBadRequest)
			return
		}
		cl := Client{
			ID:        req.ID,
			Email:     req.Email,
			InboundID: req.InboundID,
			ExpiresAt: expiresAt.UTC(),
			Enabled:   req.Enabled,
			CreatedAt: time.Now().UTC(),
		}
		if cl.ID == "" {
			cl.ID = newID("cl")
		}
		if err := s.store.UpsertClient(cl); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, cl)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleClientByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/clients/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodPut:
		var req struct {
			Email     string `json:"email"`
			InboundID string `json:"inbound_id"`
			ExpiresAt string `json:"expires_at"`
			Enabled   bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		expiresAt, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			http.Error(w, "expires_at must be RFC3339", http.StatusBadRequest)
			return
		}
		cl := Client{
			ID:        id,
			Email:     req.Email,
			InboundID: req.InboundID,
			ExpiresAt: expiresAt.UTC(),
			Enabled:   req.Enabled,
			CreatedAt: time.Now().UTC(),
		}
		if cl.Email == "" || cl.InboundID == "" {
			http.Error(w, "email and inbound_id are required", http.StatusBadRequest)
			return
		}
		if err := s.store.UpsertClient(cl); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, cl)
	case http.MethodDelete:
		if err := s.store.DeleteClient(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "deleted"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.cfg.EnableServiceOp {
		http.Error(w, "service operations disabled", http.StatusForbidden)
		return
	}
	action := strings.TrimPrefix(r.URL.Path, "/api/service/")
	switch action {
	case "start", "stop", "restart", "status":
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}
	out, err := exec.Command("systemctl", action, s.cfg.ServiceName).CombinedOutput()
	writeJSON(w, map[string]any{
		"action":  action,
		"service": s.cfg.ServiceName,
		"output":  string(out),
		"error":   errString(err),
	})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	lines := 200
	if q := r.URL.Query().Get("lines"); q != "" {
		n, err := strconv.Atoi(q)
		if err == nil && n > 0 && n <= 2000 {
			lines = n
		}
	}
	text, err := tailFile(s.cfg.ServerLogPath, lines)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"text": text})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.store.Snapshot().Settings)
	case http.MethodPut:
		var req Settings
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.store.UpdateSettings(req); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, req)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSettingsReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	def := DefaultSettings()
	if err := s.store.UpdateSettings(def); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, def)
}

func (s *Server) authOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := s.readSessionToken(r)
		if token == "" || !s.auth.Validate(token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *Server) readSessionToken(r *http.Request) string {
	c, err := r.Cookie("dsp_session")
	if err != nil {
		return ""
	}
	return c.Value
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func newID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func tailFile(path string, maxLines int) (string, error) {
	if path == "" {
		return "", fmt.Errorf("server_log_path is empty")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) <= maxLines {
		return string(b), nil
	}
	return strings.Join(lines[len(lines)-maxLines:], "\n"), nil
}
