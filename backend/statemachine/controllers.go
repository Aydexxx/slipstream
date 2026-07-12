package statemachine

import (
	"context"

	"slipstream/backend/fastmode"
	"slipstream/backend/privatemode"
)

// FastController is the subset of *fastmode.Controller the Manager and the
// app layer depend on. It exists so Manager can be unit-tested against a
// hand-rolled fake without touching real DNS, and so app.go can reach
// config-only operations (which aren't mode transitions) through Manager.Fast()
// without a direct dependency on the concrete controller type.
type FastController interface {
	Start(mode fastmode.Mode, domains []string) error
	Stop() error
	Status() fastmode.Status
	SetEmitter(fastmode.Emitter)
	DNSBackupPending() bool
	Shutdown()

	Presets() map[string][]string
	LoadCustomDomains() ([]string, error)
	SaveCustomDomains(domains []string) error
}

// PrivateController is the subset of *privatemode.Controller the Manager and
// the app layer depend on. It exists so Manager can be unit-tested against a
// hand-rolled fake without touching the real WFP engine or AmneziaWG
// service, and so app.go can reach config-only operations through
// Manager.Private() without a direct dependency on the concrete controller
// type.
type PrivateController interface {
	Connect() error
	Disconnect() error
	Status() privatemode.Status
	SetEmitter(privatemode.Emitter)
	KillSwitchArmed() bool
	DisarmKillSwitch() error
	Shutdown()

	ImportConfig(raw string) (privatemode.Summary, error)
	HasConfig() bool
	ConfigSummary() (privatemode.Summary, error)
	DeleteConfig() error

	ExternalIP(ctx context.Context) (string, error)
}

var (
	_ FastController    = (*fastmode.Controller)(nil)
	_ PrivateController = (*privatemode.Controller)(nil)
)
