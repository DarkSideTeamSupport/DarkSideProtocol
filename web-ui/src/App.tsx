import { useCallback, useEffect, useMemo, useState } from "react";
import { api } from "./api";
import type { Client, Inbound, Settings, StatsResponse } from "./types";

type Tab = "dashboard" | "inbounds" | "clients" | "settings" | "service" | "logs";

const navItems: Array<{ id: Tab; title: string }> = [
  { id: "dashboard", title: "Dashboard" },
  { id: "inbounds", title: "Inbounds" },
  { id: "clients", title: "Clients" },
  { id: "settings", title: "Settings" },
  { id: "service", title: "Service" },
  { id: "logs", title: "Logs" }
];

export function App() {
  const [isAuth, setIsAuth] = useState(false);
  const [tab, setTab] = useState<Tab>("dashboard");
  const [toast, setToast] = useState("");
  const [error, setError] = useState("");
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [stats, setStats] = useState<StatsResponse | null>(null);
  const [inbounds, setInbounds] = useState<Inbound[]>([]);
  const [clients, setClients] = useState<Client[]>([]);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [logs, setLogs] = useState("");
  const [inboundForm, setInboundForm] = useState<Partial<Inbound>>({
    name: "",
    listen: ":18443",
    transport: "tcp",
    description: "",
    enabled: true
  });
  const [clientForm, setClientForm] = useState<Partial<Client>>({
    email: "",
    inbound_id: "",
    expires_at: "",
    enabled: true
  });

  const notify = useCallback((text: string) => {
    setToast(text);
    window.setTimeout(() => setToast(""), 2600);
  }, []);

  const withErrorToast = useCallback(
    async (fn: () => Promise<void>) => {
      try {
        await fn();
      } catch (err) {
        notify(String((err as Error).message || err));
      }
    },
    [notify]
  );

  const loadAll = useCallback(async () => {
    const [s, ib, cl, stg] = await Promise.all([
      api<StatsResponse>("/api/stats"),
      api<Inbound[] | null>("/api/inbounds"),
      api<Client[] | null>("/api/clients"),
      api<Settings>("/api/settings")
    ]);
    setStats(s);
    setInbounds(Array.isArray(ib) ? ib : []);
    setClients(Array.isArray(cl) ? cl : []);
    setSettings(stg);
  }, []);

  useEffect(() => {
    api<{ authorized: boolean }>("/api/session")
      .then(async (res) => {
        if (!res.authorized) return;
        setIsAuth(true);
        await loadAll();
      })
      .catch(() => undefined);
  }, [loadAll]);

  const activeInboundIds = useMemo(() => new Set(inbounds.map((v) => v.id)), [inbounds]);

  async function login() {
    setError("");
    try {
      await api("/api/login", "POST", { username, password });
      setIsAuth(true);
      await loadAll();
      notify("Вход выполнен");
    } catch (err) {
      setError(String((err as Error).message || err));
    }
  }

  async function logout() {
    await api("/api/logout", "POST");
    setIsAuth(false);
    setPassword("");
  }

  async function createInbound() {
    if (!inboundForm.name?.trim() || !inboundForm.listen?.trim() || !inboundForm.transport?.trim()) {
      notify("Заполни обязательные поля: name, listen, transport");
      return;
    }
    await api("/api/inbounds", "POST", inboundForm);
    const data = await api<Inbound[] | null>("/api/inbounds");
    setInbounds(Array.isArray(data) ? data : []);
    notify("Inbound добавлен");
  }

  async function removeInbound(id: string) {
    await api(`/api/inbounds/${encodeURIComponent(id)}`, "DELETE");
    const data = await api<Inbound[] | null>("/api/inbounds");
    setInbounds(Array.isArray(data) ? data : []);
    notify("Inbound удален");
  }

  async function createClient() {
    if (!clientForm.email?.trim() || !clientForm.inbound_id?.trim() || !clientForm.expires_at?.trim()) {
      notify("Заполни обязательные поля клиента: email, inbound, expires");
      return;
    }
    await api("/api/clients", "POST", clientForm);
    const data = await api<Client[] | null>("/api/clients");
    setClients(Array.isArray(data) ? data : []);
    notify("Клиент добавлен");
  }

  async function removeClient(id: string) {
    await api(`/api/clients/${encodeURIComponent(id)}`, "DELETE");
    const data = await api<Client[] | null>("/api/clients");
    setClients(Array.isArray(data) ? data : []);
    notify("Клиент удален");
  }

  async function saveSettings() {
    if (!settings) return;
    await api("/api/settings", "PUT", settings);
    setSettings(await api<Settings>("/api/settings"));
    notify("Настройки сохранены");
  }

  async function resetSettings() {
    await api("/api/settings/reset", "POST");
    setSettings(await api<Settings>("/api/settings"));
    notify("Настройки сброшены");
  }

  async function serviceAction(action: string) {
    const out = await api<{ output?: string }>(`/api/service/${action}`, "POST");
    notify((out && out.output) || `Service: ${action}`);
  }

  async function refreshLogs() {
    const out = await api<{ text: string }>("/api/logs?lines=300");
    setLogs(out.text || "");
  }

  if (!isAuth) {
    return (
      <div className="login-wrap">
        <div className="card login-card">
          <h2 className="title">DarkSide Panel</h2>
          <p className="muted">Панель управления протоколом и пользователями</p>
          <div className="field">
            <label>Логин</label>
            <input value={username} onChange={(e) => setUsername(e.target.value)} />
          </div>
          <div className="field">
            <label>Пароль</label>
            <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
          </div>
          <button className="btn-primary" onClick={login}>Войти</button>
          {error ? <div className="error-box">{error}</div> : null}
        </div>
      </div>
    );
  }

  return (
    <div className="layout">
      <aside className="sidebar">
        <div className="brand">{settings?.panel.site_title || "DarkSide Panel"}</div>
        <div className="brand-sub">protocol control center</div>
        {navItems.map((item) => (
          <button
            key={item.id}
            className={`nav-btn ${tab === item.id ? "active" : ""}`}
            onClick={() => setTab(item.id)}
          >
            {item.title}
          </button>
        ))}
        <button className="nav-btn danger" onClick={() => void withErrorToast(logout)}>Выход</button>
      </aside>

      <main className="main">
        <div className="topbar">
          <h2 className="title">DarkSide Control</h2>
          <button className="btn-muted" onClick={() => void withErrorToast(loadAll)}>Обновить</button>
        </div>

        {tab === "dashboard" && stats ? (
          <div className="card">
            <h3 className="section-title">Состояние панели</h3>
            <div className="grid-4">
              <Metric label="Inbounds" value={stats.stats.inbounds} />
              <Metric label="Clients" value={stats.stats.clients} />
              <Metric label="Goroutines" value={stats.stats.go_routines} />
              <Metric label="Requests" value={stats.request_count} />
            </div>
          </div>
        ) : null}

        {tab === "inbounds" ? (
          <div className="card">
            <h3 className="section-title">Inbounds</h3>
            <div className="row">
              <LabeledInput label="Имя" value={inboundForm.name || ""} onChange={(v) => setInboundForm({ ...inboundForm, name: v })} />
              <LabeledInput label="Порт/адрес" value={inboundForm.listen || ""} onChange={(v) => setInboundForm({ ...inboundForm, listen: v })} />
              <div className="field">
                <label>Транспорт</label>
                <select value={inboundForm.transport || "tcp"} onChange={(e) => setInboundForm({ ...inboundForm, transport: e.target.value })}>
                  <option value="tcp">tcp</option>
                  <option value="udp">udp</option>
                </select>
              </div>
              <LabeledInput label="Описание" value={inboundForm.description || ""} onChange={(v) => setInboundForm({ ...inboundForm, description: v })} />
              <button className="btn-primary" onClick={() => void withErrorToast(createInbound)}>Добавить</button>
            </div>
            <table>
              <thead>
                <tr>
                  <th>ID</th><th>Name</th><th>Listen</th><th>Transport</th><th>Status</th><th></th>
                </tr>
              </thead>
              <tbody>
                {inbounds.map((v) => (
                  <tr key={v.id}>
                    <td>{v.id}</td>
                    <td>{v.name}</td>
                    <td>{v.listen}</td>
                    <td><span className="chip">{v.transport}</span></td>
                    <td><span className={`chip ${v.enabled ? "chip-green" : "chip-red"}`}>{v.enabled ? "enabled" : "disabled"}</span></td>
                    <td><button className="danger" onClick={() => void withErrorToast(() => removeInbound(v.id))}>Удалить</button></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}

        {tab === "clients" ? (
          <div className="card">
            <h3 className="section-title">Clients</h3>
            <div className="row">
              <LabeledInput label="Email" value={clientForm.email || ""} onChange={(v) => setClientForm({ ...clientForm, email: v })} />
              <div className="field">
                <label>Inbound</label>
                <select value={clientForm.inbound_id || ""} onChange={(e) => setClientForm({ ...clientForm, inbound_id: e.target.value })}>
                  <option value="">Выбери inbound</option>
                  {inbounds.map((inb) => (
                    <option key={inb.id} value={inb.id}>{inb.name} ({inb.id})</option>
                  ))}
                </select>
              </div>
              <LabeledInput label="Expires (RFC3339)" value={clientForm.expires_at || ""} onChange={(v) => setClientForm({ ...clientForm, expires_at: v })} />
              <button className="btn-primary" onClick={() => void withErrorToast(createClient)}>Добавить</button>
            </div>
            <table>
              <thead>
                <tr>
                  <th>ID</th><th>Email</th><th>Inbound</th><th>Expires</th><th>Status</th><th></th>
                </tr>
              </thead>
              <tbody>
                {clients.map((v) => (
                  <tr key={v.id}>
                    <td>{v.id}</td>
                    <td>{v.email}</td>
                    <td>{activeInboundIds.has(v.inbound_id) ? v.inbound_id : `${v.inbound_id} (missing)`}</td>
                    <td>{v.expires_at}</td>
                    <td><span className={`chip ${v.enabled ? "chip-green" : "chip-red"}`}>{v.enabled ? "enabled" : "disabled"}</span></td>
                    <td><button className="danger" onClick={() => void withErrorToast(() => removeClient(v.id))}>Удалить</button></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}

        {tab === "settings" && settings ? (
          <div className="card">
            <h3 className="section-title">Settings</h3>
            <div className="row">
              <LabeledInput label="Site title" value={settings.panel.site_title} onChange={(v) => setSettings({ ...settings, panel: { ...settings.panel, site_title: v } })} />
              <LabeledInput label="Language" value={settings.panel.default_lang} onChange={(v) => setSettings({ ...settings, panel: { ...settings.panel, default_lang: v } })} />
              <LabeledInput label="Session hours" type="number" value={String(settings.panel.session_hours)} onChange={(v) => setSettings({ ...settings, panel: { ...settings.panel, session_hours: Number(v) } })} />
              <LabeledInput label="Timezone" value={settings.panel.timezone} onChange={(v) => setSettings({ ...settings, panel: { ...settings.panel, timezone: v } })} />
            </div>
            <div className="row">
              <LabeledInput label="Default TCP port" type="number" value={String(settings.transport.default_port_tcp)} onChange={(v) => setSettings({ ...settings, transport: { ...settings.transport, default_port_tcp: Number(v) } })} />
              <LabeledInput label="Default UDP port" type="number" value={String(settings.transport.default_port_udp)} onChange={(v) => setSettings({ ...settings, transport: { ...settings.transport, default_port_udp: Number(v) } })} />
              <LabeledInput label="Allowed CIDR" value={settings.security.allowed_cidr} onChange={(v) => setSettings({ ...settings, security: { ...settings.security, allowed_cidr: v } })} />
            </div>
            <div className="row">
              <button className="btn-primary" onClick={() => void withErrorToast(saveSettings)}>Сохранить</button>
              <button className="danger" onClick={() => void withErrorToast(resetSettings)}>Сбросить</button>
            </div>
          </div>
        ) : null}

        {tab === "service" ? (
          <div className="card">
            <h3 className="section-title">Service</h3>
            <div className="row">
              <button className="btn-muted" onClick={() => void withErrorToast(() => serviceAction("status"))}>Status</button>
              <button className="btn-primary" onClick={() => void withErrorToast(() => serviceAction("start"))}>Start</button>
              <button className="btn-primary" onClick={() => void withErrorToast(() => serviceAction("restart"))}>Restart</button>
              <button className="danger" onClick={() => void withErrorToast(() => serviceAction("stop"))}>Stop</button>
            </div>
          </div>
        ) : null}

        {tab === "logs" ? (
          <div className="card">
            <h3 className="section-title">Logs</h3>
            <button className="btn-muted" onClick={() => void withErrorToast(refreshLogs)}>Обновить логи</button>
            <pre>{logs}</pre>
          </div>
        ) : null}
      </main>
      {toast ? <div className="toast">{toast}</div> : null}
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="metric">
      <div className="metric-label">{label}</div>
      <div className="metric-value">{value}</div>
    </div>
  );
}

function LabeledInput(props: { label: string; value: string; onChange: (v: string) => void; type?: string }) {
  return (
    <div className="field">
      <label>{props.label}</label>
      <input
        type={props.type || "text"}
        value={props.value}
        onChange={(e) => props.onChange(e.target.value)}
      />
    </div>
  );
}
