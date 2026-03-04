package panel

import "time"

type Inbound struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Listen      string    `json:"listen"`
	Transport   string    `json:"transport"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	Description string    `json:"description"`
}

type Client struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	InboundID string    `json:"inbound_id"`
	ExpiresAt time.Time `json:"expires_at"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

type State struct {
	Inbounds []Inbound `json:"inbounds"`
	Clients  []Client  `json:"clients"`
	Settings Settings  `json:"settings"`
}

type Stats struct {
	UptimeSeconds int64  `json:"uptime_seconds"`
	GoRoutines    int    `json:"go_routines"`
	MemoryAlloc   uint64 `json:"memory_alloc"`
	Inbounds      int    `json:"inbounds"`
	Clients       int    `json:"clients"`
}

type Settings struct {
	Panel    PanelSettings    `json:"panel"`
	Transport TransportSettings `json:"transport"`
	Obfs     ObfsSettings     `json:"obfs"`
	Security SecuritySettings `json:"security"`
	Traffic  TrafficSettings  `json:"traffic"`
}

type PanelSettings struct {
	SiteTitle     string `json:"site_title"`
	DefaultLang   string `json:"default_lang"`
	SessionHours  int    `json:"session_hours"`
	AllowSignup   bool   `json:"allow_signup"`
	Timezone      string `json:"timezone"`
}

type TransportSettings struct {
	DefaultInboundTransport string `json:"default_inbound_transport"`
	DefaultPortTCP          int    `json:"default_port_tcp"`
	DefaultPortUDP          int    `json:"default_port_udp"`
	EnableTCP               bool   `json:"enable_tcp"`
	EnableUDP               bool   `json:"enable_udp"`
	EnableFallback          bool   `json:"enable_fallback"`
}

type ObfsSettings struct {
	Enabled     bool `json:"enabled"`
	MaxPadding  int  `json:"max_padding"`
	MaxJitterMS int  `json:"max_jitter_ms"`
}

type SecuritySettings struct {
	EnableIPLimit   bool   `json:"enable_ip_limit"`
	MaxIPsPerClient int    `json:"max_ips_per_client"`
	AllowedCIDR     string `json:"allowed_cidr"`
	BlockPrivateIPs bool   `json:"block_private_ips"`
}

type TrafficSettings struct {
	EnableLimit         bool  `json:"enable_limit"`
	DefaultClientGB     int64 `json:"default_client_gb"`
	ResetEveryDays      int   `json:"reset_every_days"`
	EnableAutoSuspend   bool  `json:"enable_auto_suspend"`
}

func DefaultSettings() Settings {
	return Settings{
		Panel: PanelSettings{
			SiteTitle:    "DarkSide Panel",
			DefaultLang:  "ru",
			SessionHours: 24,
			AllowSignup:  false,
			Timezone:     "UTC",
		},
		Transport: TransportSettings{
			DefaultInboundTransport: "tcp",
			DefaultPortTCP:          18443,
			DefaultPortUDP:          18080,
			EnableTCP:               true,
			EnableUDP:               true,
			EnableFallback:          true,
		},
		Obfs: ObfsSettings{
			Enabled:     true,
			MaxPadding:  64,
			MaxJitterMS: 25,
		},
		Security: SecuritySettings{
			EnableIPLimit:   false,
			MaxIPsPerClient: 2,
			AllowedCIDR:     "0.0.0.0/0",
			BlockPrivateIPs: false,
		},
		Traffic: TrafficSettings{
			EnableLimit:       false,
			DefaultClientGB:   100,
			ResetEveryDays:    30,
			EnableAutoSuspend: false,
		},
	}
}
