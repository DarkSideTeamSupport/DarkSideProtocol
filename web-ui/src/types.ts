export type StatsResponse = {
  stats: {
    uptime_seconds: number;
    go_routines: number;
    memory_alloc: number;
    inbounds: number;
    clients: number;
  };
  request_count: number;
};

export type Inbound = {
  id: string;
  name: string;
  listen: string;
  transport: string;
  enabled: boolean;
  description: string;
  created_at?: string;
};

export type Client = {
  id: string;
  email: string;
  inbound_id: string;
  expires_at: string;
  enabled: boolean;
  created_at?: string;
};

export type Settings = {
  panel: {
    site_title: string;
    default_lang: string;
    session_hours: number;
    allow_signup: boolean;
    timezone: string;
  };
  transport: {
    default_inbound_transport: string;
    default_port_tcp: number;
    default_port_udp: number;
    enable_tcp: boolean;
    enable_udp: boolean;
    enable_fallback: boolean;
  };
  obfs: {
    enabled: boolean;
    max_padding: number;
    max_jitter_ms: number;
  };
  security: {
    enable_ip_limit: boolean;
    max_ips_per_client: number;
    allowed_cidr: string;
    block_private_ips: boolean;
  };
  traffic: {
    enable_limit: boolean;
    default_client_gb: number;
    reset_every_days: number;
    enable_auto_suspend: boolean;
  };
};
