package policy

import "time"

type Mode string

const (
	ModeUDP Mode = "udp"
	ModeTCP Mode = "tcp"
)

type Engine struct {
	current      Mode
	failuresUDP  int
	lastSwitchAt time.Time
}

func New(defaultMode Mode) *Engine {
	return &Engine{
		current:      defaultMode,
		lastSwitchAt: time.Now(),
	}
}

func (e *Engine) Current() Mode {
	return e.current
}

func (e *Engine) ReportUDPFault() {
	e.failuresUDP++
	if e.failuresUDP >= 3 && time.Since(e.lastSwitchAt) > 2*time.Second {
		e.current = ModeTCP
		e.lastSwitchAt = time.Now()
	}
}

func (e *Engine) ReportUDPHealthy() {
	e.failuresUDP = 0
	if e.current != ModeUDP && time.Since(e.lastSwitchAt) > 10*time.Second {
		e.current = ModeUDP
		e.lastSwitchAt = time.Now()
	}
}
