package policy

import "time"

type Mode string

const (
	ModeUDP Mode = "udp"
	ModeTCP Mode = "tcp"
)

type Engine struct {
	current             Mode
	failuresUDP         int
	consecutiveFailures int
	lastSwitchAt        time.Time
	lastRTT             time.Duration
	profile             string
}

func New(defaultMode Mode) *Engine {
	return &Engine{
		current:      defaultMode,
		lastSwitchAt: time.Now(),
		profile:      "stealth",
	}
}

func (e *Engine) Current() Mode {
	return e.current
}

func (e *Engine) ReportUDPFault() {
	e.failuresUDP++
	e.consecutiveFailures++
	e.updateProfile()
	if e.failuresUDP >= 3 && time.Since(e.lastSwitchAt) > 2*time.Second {
		e.current = ModeTCP
		e.lastSwitchAt = time.Now()
	}
}

func (e *Engine) ReportUDPHealthy() {
	e.failuresUDP = 0
	e.consecutiveFailures = 0
	e.updateProfile()
	if e.current != ModeUDP && time.Since(e.lastSwitchAt) > 10*time.Second {
		e.current = ModeUDP
		e.lastSwitchAt = time.Now()
	}
}

func (e *Engine) Observe(ok bool, rtt time.Duration) {
	if ok {
		e.failuresUDP = 0
		e.consecutiveFailures = 0
		if rtt > 0 {
			e.lastRTT = rtt
		}
		if e.current != ModeUDP && e.lastRTT > 0 && e.lastRTT < 250*time.Millisecond && time.Since(e.lastSwitchAt) > 8*time.Second {
			e.current = ModeUDP
			e.lastSwitchAt = time.Now()
		}
	} else {
		e.consecutiveFailures++
		e.failuresUDP++
		if e.current == ModeUDP && (e.consecutiveFailures >= 2 || e.failuresUDP >= 3) && time.Since(e.lastSwitchAt) > 2*time.Second {
			e.current = ModeTCP
			e.lastSwitchAt = time.Now()
		}
	}
	e.updateProfile()
}

func (e *Engine) Profile() string {
	return e.profile
}

func (e *Engine) updateProfile() {
	if e.consecutiveFailures >= 2 {
		e.profile = "recovery"
		return
	}
	if e.lastRTT > 0 && e.lastRTT < 120*time.Millisecond && e.current == ModeUDP {
		e.profile = "aggressive"
		return
	}
	e.profile = "stealth"
}
