// Package privatemode drives Private Mode: it brings up an AmneziaWG
// (obfuscated WireGuard) userspace tunnel by installing the amneziawg.exe
// tunnel service against a wintun interface, routes all traffic through it
// (full tunnel, DNS included), monitors handshake health, and guarantees clean
// teardown — Disconnect, app exit, or a crash all end with the original routing
// table and DNS restored.
//
// The vendored amneziawg.exe is a wireguard-windows-style manager: its tunnel
// service owns wintun creation, the default route through the TUN, tunnel DNS,
// the endpoint /32 exclusion (so the handshake can still reach the VPS), and —
// critically — the reversal of all of that when the service is removed. This
// controller owns the parts around it: the DPAPI-encrypted config store, config
// parsing/validation, UAPI-based health monitoring, retry with backoff, and the
// Turkey/DPI failure message.
//
// Private Mode requires Administrator (installing a service + reconfiguring
// networking); the app already elevates at launch.
package privatemode

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"slipstream/backend/elevate"
	"slipstream/backend/engine"
	"slipstream/backend/killswitch"
)

// isElevated is a var (not a direct call) so tests can override it to
// exercise the not-elevated path without needing an actual unelevated
// process — production always uses the real check.
var isElevated = elevate.IsElevated

// State is the coarse lifecycle surfaced to the UI.
type State string

const (
	StateDisconnected  State = "disconnected"
	StateConnecting    State = "connecting"   // service up, waiting for a handshake
	StateConnected     State = "connected"    // recent handshake — traffic flows
	StateNoHandshake   State = "no-handshake" // gave up waiting (likely DPI)
	StateDisconnecting State = "disconnecting"
	StateError         State = "error" // failed to install / bring up
)

// Monitoring / retry tuning.
const (
	monitorInterval = 2 * time.Second
	// A handshake newer than this means the tunnel is genuinely up.
	handshakeFresh = 180 * time.Second
	// How long to wait for the first handshake of an attempt before doing a
	// full tunnel reinstall. amneziawg-go retries handshakes internally, so
	// this is generous.
	handshakeGrace = 20 * time.Second
	// Full reinstall retries before declaring failure (DPI/endpoint problem).
	maxReinstalls = 2
	// Bound on how long Disconnect waits for the monitor goroutine to stop.
	monitorStopGrace = 5 * time.Second
)

// turkeyMessage is shown when the handshake never completes — the signature
// symptom of DPI blocking AmneziaWG in Turkey.
const turkeyMessage = "Handshake never completed — DPI may be blocking the VPN. " +
	"Check that your obfuscation params (Jc/Jmin/Jmax/S1-4/H1-4/I1-5) match the server, " +
	"or try another endpoint."

// Emitter is invoked on every status change (wired to a Wails event).
type Emitter func(Status)

// Status is an immutable snapshot for the frontend.
type Status struct {
	State           State     `json:"state"`
	Endpoint        string    `json:"endpoint"`
	HasConfig       bool      `json:"hasConfig"`
	LastHandshake   time.Time `json:"lastHandshake"`
	HandshakeAgeSec int64     `json:"handshakeAgeSec"` // -1 if never
	Attempt         int       `json:"attempt"`
	RxBytes         int64     `json:"rxBytes"`
	TxBytes         int64     `json:"txBytes"`
	Error           string    `json:"error"`
	Since           time.Time `json:"since"`
	// KillSwitchArmed is true when WFP leak protection is blocking all
	// non-tunnel traffic. It stays armed on a dropped tunnel (fail closed).
	KillSwitchArmed bool `json:"killSwitchArmed"`
}

// Controller is the single owner of Private Mode state. All exported methods
// are safe for concurrent use.
type Controller struct {
	log     *slog.Logger
	engine  *engine.Manager
	store   *ConfigStore
	dataDir string
	ks      *killswitch.KillSwitch

	emit   Emitter
	emitMu sync.RWMutex

	tunnelOpMu sync.Mutex // serializes install/uninstall of the tunnel service

	mu              sync.Mutex
	transitioning   bool // a Connect or Disconnect is in flight
	state           State
	endpoint        string
	pinnedEndpoint  string // resolved VPS IP, substituted into the config on install
	lastHandshake   time.Time
	rx, tx          int64
	attempt         int
	lastErr         string
	since           time.Time
	connectingSince time.Time

	monitorCancel context.CancelFunc
	monitorDone   chan struct{}
}

// Options wires the controller to its dependencies.
type Options struct {
	Log     *slog.Logger
	Engine  *engine.Manager
	DataDir string // e.g. %LocalAppData%\Slipstream\private
}

// New constructs a Controller and ensures its data directory exists.
func New(cfg Options) (*Controller, error) {
	if cfg.Engine == nil {
		return nil, fmt.Errorf("privatemode: engine manager is required")
	}
	if cfg.DataDir == "" {
		return nil, fmt.Errorf("privatemode: data directory is required")
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, fmt.Errorf("privatemode: create data directory: %w", err)
	}
	return &Controller{
		log:     cfg.Log,
		engine:  cfg.Engine,
		store:   NewConfigStore(cfg.DataDir),
		dataDir: cfg.DataDir,
		ks:      killswitch.New(filepath.Join(cfg.DataDir, "killswitch.marker"), cfg.Log),
		state:   StateDisconnected,
	}, nil
}

// SetEmitter registers the status-change callback.
func (c *Controller) SetEmitter(e Emitter) {
	c.emitMu.Lock()
	c.emit = e
	c.emitMu.Unlock()
}

func (c *Controller) awgExe() string { return c.engine.AmneziaWGPath() }
func (c *Controller) confPath() string {
	return filepath.Join(c.dataDir, tunnelName+".conf")
}

// --- config management -------------------------------------------------------

// ImportConfig validates and stores (DPAPI-encrypted) the user's Phase-4
// AmneziaWG config, returning a key-free summary for the UI.
func (c *Controller) ImportConfig(raw string) (Summary, error) {
	cfg, err := c.store.Import(raw)
	if err != nil {
		return Summary{}, err
	}
	c.mu.Lock()
	c.endpoint = cfg.Endpoint
	c.mu.Unlock()
	c.emitStatus()
	return cfg.Summary(), nil
}

// HasConfig reports whether a config has been imported.
func (c *Controller) HasConfig() bool { return c.store.Exists() }

// ConfigSummary returns the key-free summary of the imported config.
func (c *Controller) ConfigSummary() (Summary, error) {
	cfg, err := c.store.Load()
	if err != nil {
		return Summary{}, err
	}
	return cfg.Summary(), nil
}

// DeleteConfig removes the stored config. It refuses while connected so the UI
// can't leave a running tunnel with no config to manage it.
func (c *Controller) DeleteConfig() error {
	c.mu.Lock()
	st := c.state
	c.mu.Unlock()
	if st != StateDisconnected && st != StateError && st != StateNoHandshake {
		return fmt.Errorf("Disconnect Private Mode before deleting its config")
	}
	if err := c.store.Delete(); err != nil {
		return err
	}
	c.mu.Lock()
	c.endpoint = ""
	c.mu.Unlock()
	c.emitStatus()
	return nil
}

// --- status ------------------------------------------------------------------

// Status returns a snapshot of the current state.
func (c *Controller) Status() Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.snapshotLocked()
}

func (c *Controller) snapshotLocked() Status {
	age := int64(-1)
	if !c.lastHandshake.IsZero() {
		age = int64(time.Since(c.lastHandshake).Seconds())
	}
	return Status{
		State:           c.state,
		Endpoint:        c.endpoint,
		HasConfig:       c.store.Exists(),
		LastHandshake:   c.lastHandshake,
		HandshakeAgeSec: age,
		Attempt:         c.attempt,
		RxBytes:         c.rx,
		TxBytes:         c.tx,
		Error:           c.lastErr,
		Since:           c.since,
		KillSwitchArmed: c.ks.IsArmed(),
	}
}

func (c *Controller) emitStatus() {
	c.mu.Lock()
	s := c.snapshotLocked()
	c.mu.Unlock()
	c.emitMu.RLock()
	e := c.emit
	c.emitMu.RUnlock()
	if e != nil {
		e(s)
	}
}

// --- connect / disconnect ----------------------------------------------------

// Connect brings up the tunnel: it installs the AmneziaWG tunnel service from
// the stored config and starts health monitoring. It returns once the service
// is installed (state = connecting); the monitor drives the transition to
// connected, or to no-handshake with the Turkey/DPI message after retries.
func (c *Controller) Connect() error {
	if !isElevated() {
		return fmt.Errorf("Private Mode needs Administrator. Restart Slipstream and approve the User Account Control prompt")
	}

	c.mu.Lock()
	if c.transitioning {
		c.mu.Unlock()
		return fmt.Errorf("Private Mode is already changing state; try again in a moment")
	}
	if c.state == StateConnected || c.state == StateConnecting {
		c.mu.Unlock()
		return fmt.Errorf("Private Mode is already connected")
	}
	if !c.store.Exists() {
		c.mu.Unlock()
		return fmt.Errorf("No Private Mode config imported — import your AmneziaWG config first")
	}
	now := time.Now()
	c.transitioning = true
	c.state = StateConnecting
	c.attempt = 0
	c.lastErr = ""
	c.lastHandshake = time.Time{}
	c.rx, c.tx = 0, 0
	c.since = now
	c.connectingSince = now
	c.mu.Unlock()
	c.emitStatus()

	if err := c.startTunnel(); err != nil {
		c.failStart(err)
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	c.mu.Lock()
	c.monitorCancel = cancel
	c.monitorDone = done
	c.transitioning = false
	c.mu.Unlock()
	go c.monitor(ctx, done)
	c.emitStatus()
	return nil
}

// startTunnel verifies the engine, arms leak protection, installs the tunnel
// service, and then permits the tunnel adapter. Called with transitioning held.
// The kill switch is armed *before* the tunnel comes up, so the connect window
// itself can't leak: only the handshake to the VPS can leave until the tunnel
// adapter is up and explicitly permitted.
func (c *Controller) startTunnel() error {
	if err := c.engine.Verify(engine.ModePrivate); err != nil {
		return err
	}
	cfg, err := c.store.Load()
	if err != nil {
		return err
	}

	// Resolve the VPS endpoint to an IP while DNS still works, and pin it into
	// the config so the tunnel service never needs DNS (which the kill switch
	// blocks) to (re)connect.
	ip, port, err := resolveEndpoint(cfg.Endpoint)
	if err != nil {
		return fmt.Errorf("resolve VPS endpoint %q: %w", cfg.Endpoint, err)
	}

	c.mu.Lock()
	c.endpoint = cfg.Endpoint
	c.pinnedEndpoint = ip.String()
	c.mu.Unlock()

	// Arm the kill switch first — fail closed from this point on.
	if err := c.ks.Arm(killswitch.Params{EndpointIP: ip, EndpointPort: port, AllowLANDHCP: true}); err != nil {
		return fmt.Errorf("arm kill switch: %w", err)
	}
	c.emitStatus()

	c.tunnelOpMu.Lock()
	err = c.installFromStore(context.Background())
	c.tunnelOpMu.Unlock()
	if err != nil {
		return classifyInstallError(err)
	}

	// Once the tunnel adapter exists, permit traffic through it.
	luid, err := c.waitTunnelLUID(8 * time.Second)
	if err != nil {
		return fmt.Errorf("tunnel adapter did not appear: %w", err)
	}
	if err := c.ks.AllowTunnel(luid); err != nil {
		return fmt.Errorf("permit tunnel in kill switch: %w", err)
	}

	if c.log != nil {
		c.log.Info("private mode tunnel installed", "endpoint", cfg.Endpoint, "pinnedIP", ip.String(), "fullTunnel", cfg.FullTunnel)
	}
	return nil
}

// resolveEndpoint splits a "host:port" endpoint and resolves host to a single
// IP (preferring IPv4). An IP-literal host is returned as-is.
func resolveEndpoint(endpoint string) (netip.Addr, uint16, error) {
	host, portStr, err := net.SplitHostPort(endpoint)
	if err != nil {
		return netip.Addr{}, 0, err
	}
	p, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return netip.Addr{}, 0, fmt.Errorf("invalid port %q", portStr)
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		return ip.Unmap(), uint16(p), nil
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return netip.Addr{}, 0, fmt.Errorf("could not resolve host %q: %w", host, err)
	}
	chosen := ips[0]
	for _, cand := range ips {
		if cand.To4() != nil {
			chosen = cand
			break
		}
	}
	addr, ok := netip.AddrFromSlice(chosen)
	if !ok {
		return netip.Addr{}, 0, fmt.Errorf("unusable resolved IP for %q", host)
	}
	return addr.Unmap(), uint16(p), nil
}

// waitTunnelLUID polls for the tunnel adapter's LUID, which appears a moment
// after the service starts creating the wintun device.
func (c *Controller) waitTunnelLUID(timeout time.Duration) (uint64, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		luid, err := killswitch.InterfaceLUID(tunnelName)
		if err == nil && luid != 0 {
			return luid, nil
		}
		lastErr = err
		time.Sleep(300 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("timed out waiting for adapter %q", tunnelName)
	}
	return 0, lastErr
}

// installFromStore decrypts the config, ensures a tunnel DNS, writes a
// short-lived plaintext .conf, removes any leftover service, installs the new
// one, and shreds the plaintext file. The caller must hold tunnelOpMu.
func (c *Controller) installFromStore(ctx context.Context) error {
	raw, err := c.store.LoadRaw()
	if err != nil {
		return err
	}
	raw = ensureDNS(raw, "1.1.1.1")

	// Pin the endpoint to the IP resolved at connect time, so the service never
	// needs DNS (blocked by the kill switch) to reach the VPS on a reconnect.
	c.mu.Lock()
	pinned := c.pinnedEndpoint
	c.mu.Unlock()
	if pinned != "" {
		raw = pinEndpoint(raw, pinned)
	}

	// Clear any leftover service first so install can't collide with it.
	if err := uninstallTunnel(ctx, c.awgExe()); err != nil && c.log != nil {
		c.log.Warn("removing leftover tunnel before install", "error", err)
	}
	if err := waitServiceGone(ctx, 5*time.Second); err != nil && c.log != nil {
		c.log.Warn("previous tunnel service slow to disappear", "error", err)
	}

	path := c.confPath()
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		return fmt.Errorf("write tunnel config: %w", err)
	}
	// The plaintext config (with the private key) is copied into the service's
	// own encrypted store during install; shred ours as soon as we're done.
	defer shred(path)

	return installTunnel(ctx, c.awgExe(), path)
}

// failStart rolls back a failed Connect: tear down any partial tunnel, disarm
// the kill switch (a failed *initial* connect should not strand the user
// offline), and mark the error. No monitor was started yet.
func (c *Controller) failStart(err error) {
	c.tunnelOpMu.Lock()
	_ = uninstallTunnel(context.Background(), c.awgExe())
	c.tunnelOpMu.Unlock()
	if derr := c.ks.Disarm(); derr != nil && c.log != nil {
		c.log.Error("kill switch disarm during failed start reported errors", "error", derr)
	}
	c.mu.Lock()
	c.state = StateError
	c.lastErr = err.Error()
	c.transitioning = false
	c.mu.Unlock()
	c.emitStatus()
	if c.log != nil {
		c.log.Error("private mode failed to start", "error", err)
	}
}

// monitor polls the tunnel's UAPI for handshake health and drives the state
// machine: connecting -> connected, connecting -> (reinstall retries) ->
// no-handshake, and connected -> connecting if the link goes stale.
func (c *Controller) monitor(ctx context.Context, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(monitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		c.mu.Lock()
		if c.state == StateDisconnecting || c.state == StateDisconnected {
			c.mu.Unlock()
			return
		}
		prevState := c.state
		connectingSince := c.connectingSince
		c.mu.Unlock()

		info, err := queryHandshake(tunnelName)
		now := time.Now()
		fresh := err == nil && info.HasPeer && !info.LastHandshake.IsZero() && info.Age(now) < handshakeFresh

		if fresh {
			c.mu.Lock()
			c.lastHandshake = info.LastHandshake
			c.rx, c.tx = info.RxBytes, info.TxBytes
			if c.state != StateConnected {
				c.state = StateConnected
				c.since = now
				if c.log != nil {
					c.log.Info("private mode connected", "endpoint", c.endpoint)
				}
			}
			c.attempt = 0
			c.lastErr = ""
			c.mu.Unlock()
			c.emitStatus()
			continue
		}

		// No fresh handshake this tick.
		if prevState == StateConnected {
			// We were up and lost it — restart the connecting cycle.
			c.mu.Lock()
			c.state = StateConnecting
			c.connectingSince = now
			c.mu.Unlock()
			c.emitStatus()
			if c.log != nil {
				c.log.Warn("private mode handshake went stale; reconnecting")
			}
			continue
		}

		// Still connecting: keep waiting within the grace window.
		if now.Sub(connectingSince) < handshakeGrace {
			if err == nil {
				c.mu.Lock()
				c.rx, c.tx = info.RxBytes, info.TxBytes
				if !info.LastHandshake.IsZero() {
					c.lastHandshake = info.LastHandshake
				}
				c.mu.Unlock()
			}
			continue
		}

		// Grace exceeded with no handshake — reinstall or give up.
		c.mu.Lock()
		c.attempt++
		attempt := c.attempt
		c.mu.Unlock()

		if attempt > maxReinstalls {
			c.giveUp()
			return
		}

		if c.log != nil {
			c.log.Warn("private mode no handshake; reinstalling tunnel", "attempt", attempt)
		}
		c.emitStatus()

		// Backoff, then a full reinstall (helps if wintun/service wedged).
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoffFor(attempt)):
		}
		c.mu.Lock()
		if c.state == StateDisconnecting || c.state == StateDisconnected {
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()

		c.tunnelOpMu.Lock()
		rerr := c.installFromStore(ctx)
		c.tunnelOpMu.Unlock()
		c.mu.Lock()
		c.connectingSince = time.Now()
		if rerr != nil {
			c.lastErr = rerr.Error()
		}
		c.mu.Unlock()
		c.emitStatus()
	}
}

// giveUp declares the DPI/endpoint failure and tears everything down — tunnel
// and kill switch — so a connection that never established doesn't leave the
// user stranded offline. (An established session that later drops stays fail
// closed; that path never reaches giveUp.)
func (c *Controller) giveUp() {
	c.mu.Lock()
	c.state = StateNoHandshake
	c.lastErr = turkeyMessage
	c.mu.Unlock()
	c.emitStatus()
	if c.log != nil {
		c.log.Error("private mode giving up: handshake never completed (possible DPI block)")
	}
	c.tunnelOpMu.Lock()
	_ = uninstallTunnel(context.Background(), c.awgExe())
	c.tunnelOpMu.Unlock()
	if err := c.ks.Disarm(); err != nil && c.log != nil {
		c.log.Error("kill switch disarm on give-up reported errors", "error", err)
	}
	c.emitStatus()
}

// Disconnect tears the tunnel down and restores normal networking. It is safe
// to call when already disconnected (best-effort cleanup) and always attempts
// the uninstall even if the monitor is slow to stop.
func (c *Controller) Disconnect() error {
	c.mu.Lock()
	if c.state == StateDisconnected {
		c.mu.Unlock()
		// Belt-and-braces: remove any tunnel we don't think we own.
		c.tunnelOpMu.Lock()
		err := uninstallTunnel(context.Background(), c.awgExe())
		c.tunnelOpMu.Unlock()
		return err
	}
	if c.transitioning {
		c.mu.Unlock()
		return fmt.Errorf("Private Mode is busy changing state; try again in a moment")
	}
	c.transitioning = true
	c.state = StateDisconnecting
	cancel := c.monitorCancel
	done := c.monitorDone
	c.mu.Unlock()
	c.emitStatus()

	// Stop the monitor so only we touch the tunnel from here.
	if cancel != nil {
		cancel()
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(monitorStopGrace):
			if c.log != nil {
				c.log.Warn("private mode monitor slow to stop; proceeding with teardown")
			}
		}
	}

	// Remove the service — this restores routing + DNS. Retry once.
	c.tunnelOpMu.Lock()
	err := uninstallTunnel(context.Background(), c.awgExe())
	if err != nil {
		err = uninstallTunnel(context.Background(), c.awgExe())
	}
	c.tunnelOpMu.Unlock()

	// Then disarm leak protection — internet is fully restored only once the
	// WFP block is gone. Do it last so traffic stays blocked until the tunnel
	// is truly torn down.
	if derr := c.ks.Disarm(); derr != nil {
		if c.log != nil {
			c.log.Error("kill switch disarm on disconnect reported errors", "error", derr)
		}
		if err == nil {
			err = derr
		}
	}

	c.mu.Lock()
	c.state = StateDisconnected
	c.monitorCancel = nil
	c.monitorDone = nil
	c.lastHandshake = time.Time{}
	c.rx, c.tx = 0, 0
	c.attempt = 0
	c.since = time.Time{}
	if err != nil {
		c.lastErr = err.Error()
	} else {
		c.lastErr = ""
	}
	c.transitioning = false
	c.mu.Unlock()
	c.emitStatus()

	if c.log != nil {
		if err != nil {
			c.log.Error("private mode disconnect reported errors", "error", err)
		} else {
			c.log.Info("private mode disconnected; networking restored")
		}
	}
	return err
}

// Shutdown is the app-exit hook: disconnect and, unconditionally, make sure no
// tunnel service is left behind (so the user's networking is never stuck in a
// tunnel after the app is gone).
func (c *Controller) Shutdown() {
	if err := c.Disconnect(); err != nil && c.log != nil {
		c.log.Error("private mode shutdown disconnect error", "error", err)
	}
	c.tunnelOpMu.Lock()
	if err := uninstallTunnel(context.Background(), c.awgExe()); err != nil && c.log != nil {
		c.log.Error("private mode shutdown teardown error", "error", err)
	}
	c.tunnelOpMu.Unlock()
	// Unconditional backstop: never leave WFP leak-protection filters behind on
	// exit (they would keep the user offline until next launch reconciles).
	if err := c.ks.Disarm(); err != nil && c.log != nil {
		c.log.Error("private mode shutdown kill switch disarm error", "error", err)
	}
}

// DisarmKillSwitch is the manual "restore internet" control (Phase 8). It
// removes the WFP leak-protection filters immediately — restoring normal
// networking — without tearing down the tunnel. Use it to recover if a dropped
// tunnel has left the machine fail-closed with no connectivity.
func (c *Controller) DisarmKillSwitch() error {
	if err := c.ks.Disarm(); err != nil {
		return err
	}
	c.emitStatus()
	if c.log != nil {
		c.log.Warn("kill switch manually disarmed by user; leak protection off")
	}
	return nil
}

// KillSwitchArmed reports whether WFP leak protection is currently active.
func (c *Controller) KillSwitchArmed() bool { return c.ks.IsArmed() }

// backoffFor returns the wait before reinstall attempt n (1-based).
func backoffFor(n int) time.Duration {
	switch {
	case n <= 1:
		return 5 * time.Second
	default:
		return 10 * time.Second
	}
}

// classifyInstallError turns a raw /installtunnelservice failure into an
// actionable message.
func classifyInstallError(err error) error {
	s := strings.ToLower(err.Error())
	switch {
	case strings.Contains(s, "access is denied") || strings.Contains(s, "denied"):
		return fmt.Errorf("Windows denied installing the tunnel service — Private Mode must run as Administrator. Restart Slipstream and approve the UAC prompt")
	case strings.Contains(s, "already exists") || strings.Contains(s, "marked for deletion"):
		return fmt.Errorf("A previous tunnel is still shutting down; wait a few seconds and try again")
	default:
		return fmt.Errorf("Could not start the AmneziaWG tunnel: %w", err)
	}
}

// shred overwrites a small file with zeros before removing it, so the plaintext
// tunnel key doesn't linger in a recoverable form. Best-effort.
func shred(path string) {
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		if f, err := os.OpenFile(path, os.O_WRONLY, 0o600); err == nil {
			_, _ = f.Write(make([]byte, info.Size()))
			_ = f.Sync()
			_ = f.Close()
		}
	}
	_ = os.Remove(path)
}

// ShredLeftoverPlaintextConfig securely removes any plaintext tunnel .conf
// left in dataDir. installFromStore writes this file (it contains the private
// key) and shreds it via defer once the service has copied it into its own
// encrypted store — but a hard kill in that window leaves the key on disk.
// This is called at start-up (and by the uninstaller) so the key never
// survives to a later session. A missing file is a no-op.
func ShredLeftoverPlaintextConfig(dataDir string, log *slog.Logger) {
	path := filepath.Join(dataDir, tunnelName+".conf")
	if _, err := os.Stat(path); err != nil {
		return // nothing to shred
	}
	if log != nil {
		log.Warn("found leftover plaintext tunnel config from a previous run; shredding")
	}
	shred(path)
}

// RecoverLeftoverTunnel is called once at start-up. If a previous run crashed
// while Private Mode was connected, its tunnel service is still running (the
// user still has working networking, through the VPN). Remove it so the app
// starts from a clean, normal-networking baseline. A clean prior shutdown
// leaves nothing, making this a no-op.
func RecoverLeftoverTunnel(awgExe string, log *slog.Logger) error {
	if !ServiceExists() {
		return nil
	}
	if log != nil {
		log.Warn("found leftover AmneziaWG tunnel from a previous run; removing to restore normal networking")
	}
	return uninstallTunnel(context.Background(), awgExe)
}
