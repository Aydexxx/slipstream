package statemachine

import (
	"slipstream/backend/fastmode"
)

// FastController is the subset of *fastmode.Controller the Manager and the
// app layer depend on. It exists so Manager can be unit-tested against a
// hand-rolled fake without touching real DNS, and so app.go can reach
// config-only operations (which aren't mode transitions) through Manager.Fast()
// without a direct dependency on the concrete controller type.
type FastController interface {
	Start(mode fastmode.Mode, strategyID string, domains []string) error
	Stop() error
	Status() fastmode.Status
	SetEmitter(fastmode.Emitter)
	DNSBackupPending() bool
	Shutdown()

	Presets() map[string][]string
	Strategies() []fastmode.StrategyInfo
	LoadCustomDomains() ([]string, error)
	SaveCustomDomains(domains []string) error
}

var _ FastController = (*fastmode.Controller)(nil)
