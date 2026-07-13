// Package statemachine is the single coordinator for Fast Mode. Fast Mode's
// own controller (backend/fastmode) is already internally correct —
// idempotent Start/Stop, crash-safe teardown, its own locking. What was
// missing is coordination: a single owner of the top-level state, persisted
// last-mode/reconnect preferences, and one unified event stream for the
// frontend.
//
// Manager is that piece. Every mode change goes through it; it verifies that
// teardown actually left clean DNS state before reporting Idle, and it emits
// a single unified Status.
package statemachine

import (
	"time"

	"slipstream/backend/fastmode"
)

// State is the coarse lifecycle surfaced to the UI.
type State string

const (
	StateIdle       State = "idle"
	StateFastActive State = "fast-active"
	StateError      State = "error"
)

// SubMode identifies which underlying controller, if any, currently owns
// the network.
type SubMode string

const (
	SubModeNone SubMode = ""
	SubModeFast SubMode = "fast"
)

// Status is an immutable snapshot of the unified state for the frontend. It
// embeds Fast Mode's own status so detail (restart count, ...) isn't lost in
// the coarse projection.
type Status struct {
	State             State            `json:"state"`
	SubMode           SubMode          `json:"subMode"`
	Transitioning     bool             `json:"transitioning"`
	Healthy           bool             `json:"healthy"`
	Error             string           `json:"error"`
	Since             time.Time        `json:"since"`
	FastStatus        *fastmode.Status `json:"fastStatus,omitempty"`
	ReconnectOnLaunch bool             `json:"reconnectOnLaunch"`
	// LastFastStrategy is the persisted desync-strategy ID the user last chose
	// for Fast Mode (settings-derived, like ReconnectOnLaunch). The frontend
	// uses it to preselect the strategy picker before Fast Mode is started;
	// while Fast Mode is active, FastStatus.Strategy carries the running one.
	LastFastStrategy string `json:"lastFastStrategy"`
}

// Emitter is invoked on every unified status change (wired to a Wails event
// in the app layer).
type Emitter func(Status)

// Settings persist across launches so the app can optionally resume the
// user's last mode.
type Settings struct {
	LastMode          SubMode  `json:"lastMode"`
	LastFastSubMode   string   `json:"lastFastSubMode,omitempty"`
	LastFastStrategy  string   `json:"lastFastStrategy,omitempty"`
	LastFastDomains   []string `json:"lastFastDomains,omitempty"`
	ReconnectOnLaunch bool     `json:"reconnectOnLaunch"`
}
