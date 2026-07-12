// Package killswitch provides Private Mode's fail-closed leak protection using
// the Windows Filtering Platform (WFP).
//
// When armed it installs WFP filters that block ALL outbound traffic (IPv4 and
// IPv6) except: the tunnel adapter, the single VPS endpoint IP:port (so the
// WireGuard handshake can complete), loopback, and DHCP (so the underlying link
// keeps its lease). Plain DNS on port 53 and every IPv6 destination are covered
// by the catch-all block, so nothing — no traffic, no DNS, no v6 — escapes over
// the real connection if the tunnel drops.
//
// It fails closed by design: filters are installed in a non-dynamic WFP session,
// so they persist even if the app is force-killed; internet stays cut until the
// app disarms or the machine reboots. A small on-disk marker lets the next
// launch reconcile (remove any leftover filters) so a hard crash can never leave
// the user permanently offline.
package killswitch

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"slipstream/backend/elevate"
)

// isElevated is a var (not a direct call) so tests can override it to
// exercise the not-elevated path without needing an actual unelevated
// process — production always uses the real check.
var isElevated = elevate.IsElevated

// KillSwitch owns the armed/disarmed lifecycle of the WFP filter set.
type KillSwitch struct {
	markerPath string
	log        *slog.Logger

	mu      sync.Mutex
	armed   bool
	params  Params
	tunLUID uint64
}

// marker is the on-disk record that the kill switch is (or was) armed. Its
// presence at launch means a previous run left filters behind.
type marker struct {
	ArmedAt      time.Time `json:"armedAt"`
	EndpointIP   string    `json:"endpointIP"`
	EndpointPort uint16    `json:"endpointPort"`
}

// New creates a KillSwitch whose reconciliation marker lives at markerPath.
func New(markerPath string, log *slog.Logger) *KillSwitch {
	return &KillSwitch{markerPath: markerPath, log: log}
}

// IsArmed reports whether the kill switch currently believes it is armed.
func (k *KillSwitch) IsArmed() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.armed
}

// Arm installs the base block + permit filters (everything except the tunnel
// adapter, which is added later via AllowTunnel). It is idempotent. The marker
// is written *before* the filters so a crash mid-arm is still reconcilable.
func (k *KillSwitch) Arm(p Params) error {
	if !isElevated() {
		return fmt.Errorf("Kill switch needs Administrator. Restart Slipstream and approve the User Account Control prompt")
	}
	if !p.EndpointIP.IsValid() {
		return fmt.Errorf("The kill switch needs a valid endpoint IP to permit the handshake")
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	if err := k.writeMarker(p); err != nil {
		return err
	}

	e, err := openEngine()
	if err != nil {
		return err
	}
	defer e.close()

	if err := e.begin(); err != nil {
		return err
	}
	if err := e.ensureProvider(); err != nil {
		e.abort()
		return err
	}
	if err := e.ensureSublayer(); err != nil {
		e.abort()
		return err
	}
	for _, spec := range baseFilterSpecs(p) {
		if err := e.replaceFilter(spec); err != nil {
			e.abort()
			return err
		}
	}
	if err := e.commit(); err != nil {
		return err
	}

	k.armed = true
	k.params = p
	if k.log != nil {
		k.log.Info("kill switch armed", "endpoint", p.EndpointIP.String(), "port", p.EndpointPort)
	}
	return nil
}

// AllowTunnel adds the permit-tunnel filters once the tunnel adapter exists.
// Until this is called, only the handshake to the endpoint can leave — which is
// exactly what we want during the connect window.
func (k *KillSwitch) AllowTunnel(luid uint64) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if !k.armed {
		return fmt.Errorf("kill switch is not armed")
	}

	e, err := openEngine()
	if err != nil {
		return err
	}
	defer e.close()

	if err := e.begin(); err != nil {
		return err
	}
	for _, spec := range tunnelFilterSpecs(luid) {
		if err := e.replaceFilter(spec); err != nil {
			e.abort()
			return err
		}
	}
	if err := e.commit(); err != nil {
		return err
	}
	k.tunLUID = luid
	if k.log != nil {
		k.log.Info("kill switch now permitting tunnel adapter", "luid", luid)
	}
	return nil
}

// Disarm removes every Slipstream WFP filter and deletes the marker, restoring
// normal networking. Safe to call when not armed.
func (k *KillSwitch) Disarm() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.disarmLocked()
}

func (k *KillSwitch) disarmLocked() error {
	e, err := openEngine()
	if err != nil {
		return err
	}
	defer e.close()

	if err := e.removeAll(); err != nil {
		return err
	}
	k.deleteMarker()
	k.armed = false
	k.tunLUID = 0
	if k.log != nil {
		k.log.Info("kill switch disarmed; normal networking restored")
	}
	return nil
}

func (k *KillSwitch) writeMarker(p Params) error {
	if err := os.MkdirAll(filepath.Dir(k.markerPath), 0o755); err != nil {
		return fmt.Errorf("kill switch marker dir: %w", err)
	}
	data, err := json.Marshal(marker{ArmedAt: time.Now(), EndpointIP: p.EndpointIP.String(), EndpointPort: p.EndpointPort})
	if err != nil {
		return err
	}
	tmp := k.markerPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write kill switch marker: %w", err)
	}
	if err := os.Rename(tmp, k.markerPath); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("commit kill switch marker: %w", err)
	}
	return nil
}

func (k *KillSwitch) deleteMarker() {
	_ = os.Remove(k.markerPath)
}

// MarkerPresent reports whether a reconciliation marker exists on disk.
func MarkerPresent(markerPath string) bool {
	_, err := os.Stat(markerPath)
	return err == nil
}

// Reconcile is called once at start-up, before the UI. It unconditionally
// removes any Slipstream WFP filters left over from a previous run (a crash or
// hard kill while armed would leave the user's internet cut) and clears the
// marker. With no leftovers it is a cheap no-op. This is the guarantee that a
// crash can never strand the user offline.
func Reconcile(markerPath string, log *slog.Logger) error {
	hadMarker := MarkerPresent(markerPath)

	e, err := openEngine()
	if err != nil {
		// If we can't open WFP we also couldn't have installed filters; nothing
		// to reconcile. Still clear the marker so we don't loop on it.
		if hadMarker {
			_ = os.Remove(markerPath)
		}
		return fmt.Errorf("kill switch reconcile: %w", err)
	}
	defer e.close()

	if err := e.removeAll(); err != nil {
		return fmt.Errorf("kill switch reconcile removeAll: %w", err)
	}
	_ = os.Remove(markerPath)

	if hadMarker && log != nil {
		log.Warn("removed leftover kill-switch filters from a previous run; internet restored")
	}
	return nil
}
