// Package statemachine is the single coordinator for Fast Mode, Private Mode,
// and the kill switch. Each mode's own controller (backend/fastmode,
// backend/privatemode) is already internally correct — idempotent
// Start/Stop, crash-safe teardown, its own locking. What was missing is
// coordination: nothing stopped both modes running at once, and the frontend
// had two independent event streams to reconcile itself.
//
// Manager is that missing piece. Every mode change goes through it; it
// enforces mutual exclusion by fully tearing down whichever mode is active
// and verifying that teardown actually left clean DNS/WFP state before
// starting the next one, and it emits a single unified Status covering both
// controllers plus the kill switch.
package statemachine

import (
	"time"

	"slipstream/backend/fastmode"
	"slipstream/backend/privatemode"
)

// State is the coarse lifecycle surfaced to the UI.
type State string

const (
	StateIdle              State = "idle"
	StateFastActive        State = "fast-active"
	StatePrivateConnecting State = "private-connecting"
	StatePrivateActive     State = "private-active"
	StateError             State = "error"
)

// SubMode identifies which underlying controller, if any, currently owns
// the network.
type SubMode string

const (
	SubModeNone    SubMode = ""
	SubModeFast    SubMode = "fast"
	SubModePrivate SubMode = "private"
)

// Status is an immutable snapshot of the unified state for the frontend. It
// embeds each sub-controller's own status so detail (handshake age, restart
// count, ...) isn't lost in the coarse projection.
type Status struct {
	State             State               `json:"state"`
	SubMode           SubMode             `json:"subMode"`
	Transitioning     bool                `json:"transitioning"`
	Healthy           bool                `json:"healthy"`
	Error             string              `json:"error"`
	Since             time.Time           `json:"since"`
	FastStatus        *fastmode.Status    `json:"fastStatus,omitempty"`
	PrivateStatus     *privatemode.Status `json:"privateStatus,omitempty"`
	KillSwitchArmed   bool                `json:"killSwitchArmed"`
	ReconnectOnLaunch bool                `json:"reconnectOnLaunch"`
}

// Emitter is invoked on every unified status change (wired to a Wails event
// in the app layer).
type Emitter func(Status)

// Settings persist across launches so the app can optionally resume the
// user's last mode.
type Settings struct {
	LastMode          SubMode  `json:"lastMode"`
	LastFastSubMode   string   `json:"lastFastSubMode,omitempty"`
	LastFastDomains   []string `json:"lastFastDomains,omitempty"`
	ReconnectOnLaunch bool     `json:"reconnectOnLaunch"`
}
