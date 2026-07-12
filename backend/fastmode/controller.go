// Package fastmode drives Fast Mode: it launches and supervises winws.exe
// (zapret) to defeat DPI via TLS ClientHello fragmentation, switches the
// system resolver to encrypted Cloudflare DoH to defeat DNS poisoning, and
// guarantees that every stop path — clean Stop, app exit, a hard kill, or a
// crash — restores the user's original DNS. No proxy and no tunnel are
// involved, so there is no measurable speed loss.
//
// Fast Mode requires Administrator (WinDivert loads a kernel driver and DNS
// is a system setting); the app already elevates at launch.
package fastmode

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"slipstream/backend/elevate"
	"slipstream/backend/engine"

	"gopkg.in/natefinch/lumberjack.v2"
)

// isElevated is a var (not a direct call) so tests can override it to
// exercise the not-elevated path without needing an actual unelevated
// process — production always uses the real check.
var isElevated = elevate.IsElevated

// State is the coarse lifecycle of Fast Mode, surfaced to the UI.
type State string

const (
	StateStopped    State = "stopped"
	StateStarting   State = "starting"
	StateRunning    State = "running"
	StateRestarting State = "restarting"
	StateStopping   State = "stopping"
	StateFailed     State = "failed"
)

// Status is an immutable snapshot of the controller for the frontend.
type Status struct {
	State    State     `json:"state"`
	Mode     Mode      `json:"mode"`
	Domains  []string  `json:"domains"`
	PID      int       `json:"pid"`
	Restarts int       `json:"restarts"`
	Error    string    `json:"error"`
	DNS      bool      `json:"dnsApplied"`
	Since    time.Time `json:"since"`
}

// Supervision tuning.
const (
	// A launch that dies within minHealthyRuntime is treated as a crash-on-
	// start (bad config, driver refused to load) rather than a transient
	// drop, and is not blindly restarted.
	minHealthyRuntime = 3 * time.Second
	// Consecutive fast failures before we give up and tear Fast Mode down.
	maxFastFails = 2
	// Cap on transient restarts (process was healthy, then exited) before we
	// stop trying, so a persistent problem can't spin forever.
	maxRestarts    = 5
	restartBackoff = 750 * time.Millisecond
	// How long Stop waits for a killed process to actually exit.
	killGrace = 5 * time.Second
)

// Emitter is an optional callback invoked on every status change (wired to a
// Wails event in the app layer).
type Emitter func(Status)

// Controller is the single owner of Fast Mode state. All exported methods are
// safe for concurrent use.
type Controller struct {
	log     *slog.Logger
	engine  *engine.Manager
	dataDir string
	dns     *dnsManager

	winwsLog *lumberjack.Logger

	emit   Emitter
	emitMu sync.RWMutex

	mu            sync.Mutex
	transitioning bool // a Start or Stop is in flight; reject re-entry
	state         State
	mode          Mode
	domains       []string
	restarts      int
	fastFails     int
	lastErr       string
	dnsApplied    bool
	since         time.Time

	// Details of the current launch, so the supervisor can relaunch.
	winwsPath string
	winwsDir  string
	winwsArgs []string

	cur *proc // currently supervised process, nil when stopped
}

// proc is one launched winws.exe instance under supervision.
type proc struct {
	cmd       *exec.Cmd
	exited    chan struct{}
	startedAt time.Time
	stderr    *ringBuffer
}

// Config wires the controller to its dependencies.
type Config struct {
	Log     *slog.Logger
	Engine  *engine.Manager
	DataDir string // e.g. %LocalAppData%\Slipstream\fastmode
	LogDir  string // where winws.log is rotated
}

// New constructs a Controller and ensures its data directory exists.
func New(cfg Config) (*Controller, error) {
	if cfg.Engine == nil {
		return nil, fmt.Errorf("fastmode: engine manager is required")
	}
	if cfg.DataDir == "" {
		return nil, fmt.Errorf("fastmode: data directory is required")
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, fmt.Errorf("fastmode: create data directory: %w", err)
	}
	logDir := cfg.LogDir
	if logDir == "" {
		logDir = cfg.DataDir
	}
	c := &Controller{
		log:     cfg.Log,
		engine:  cfg.Engine,
		dataDir: cfg.DataDir,
		dns:     newDNSManager(filepath.Join(cfg.DataDir, "dns_backup.json"), cfg.Log),
		state:   StateStopped,
		winwsLog: &lumberjack.Logger{
			Filename:   filepath.Join(logDir, "winws.log"),
			MaxSize:    5,
			MaxBackups: 3,
			MaxAge:     14,
			Compress:   true,
		},
	}
	return c, nil
}

// SetEmitter registers the status-change callback. Safe to call at any time.
func (c *Controller) SetEmitter(e Emitter) {
	c.emitMu.Lock()
	c.emit = e
	c.emitMu.Unlock()
}

// customListPath is where the user's persisted custom domains live.
func (c *Controller) customListPath() string {
	return filepath.Join(c.dataDir, "custom_domains.txt")
}

// LoadCustomDomains returns the user's saved custom domain list.
func (c *Controller) LoadCustomDomains() ([]string, error) {
	return loadCustomDomains(c.customListPath())
}

// SaveCustomDomains persists the user's custom domain list.
func (c *Controller) SaveCustomDomains(domains []string) error {
	return saveCustomDomains(c.customListPath(), domains)
}

// Presets exposes the bundled preset groups to the frontend.
func (c *Controller) Presets() map[string][]string { return Presets() }

// DNSBackupPending reports whether a DNS backup is still on disk, i.e. a
// restore to the user's original DNS is still owed. It is a live signal —
// unlike Status().DNS, which is best-effort and reset regardless of whether
// the restore actually succeeded — used by callers that need to verify
// teardown genuinely completed before proceeding.
func (c *Controller) DNSBackupPending() bool {
	return c.dns.pending()
}

// Status returns a snapshot of the current state.
func (c *Controller) Status() Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.snapshotLocked()
}

func (c *Controller) snapshotLocked() Status {
	pid := 0
	if c.cur != nil && c.cur.cmd != nil && c.cur.cmd.Process != nil {
		pid = c.cur.cmd.Process.Pid
	}
	ds := make([]string, len(c.domains))
	copy(ds, c.domains)
	return Status{
		State:    c.state,
		Mode:     c.mode,
		Domains:  ds,
		PID:      pid,
		Restarts: c.restarts,
		Error:    c.lastErr,
		DNS:      c.dnsApplied,
		Since:    c.since,
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

// Start turns Fast Mode on for the given sub-mode. For ModeCustom, domains is
// the resolved list of hosts to de-censor; it is ignored for Full and
// Discord. Start is idempotent-safe against concurrent callers and refuses to
// run without Administrator or with a failed engine-integrity check.
func (c *Controller) Start(mode Mode, domains []string) error {
	if !validMode(mode) {
		return fmt.Errorf("unknown Fast Mode sub-mode %q", mode)
	}
	if !isElevated() {
		return fmt.Errorf("Fast Mode needs Administrator. Restart Slipstream and approve the User Account Control prompt")
	}

	// Claim the transition so a second Start/Stop can't race us.
	c.mu.Lock()
	if c.transitioning {
		c.mu.Unlock()
		return fmt.Errorf("Fast Mode is already changing state; try again in a moment")
	}
	if c.state == StateRunning || c.state == StateStarting || c.state == StateRestarting {
		c.mu.Unlock()
		return fmt.Errorf("Fast Mode is already running")
	}
	c.transitioning = true
	c.state = StateStarting
	c.mode = mode
	c.restarts = 0
	c.fastFails = 0
	c.lastErr = ""
	c.mu.Unlock()
	c.emitStatus()

	if err := c.startLocked(mode, domains); err != nil {
		// startLocked already rolled back (DNS/process) on failure.
		c.mu.Lock()
		c.state = StateFailed
		c.lastErr = err.Error()
		c.transitioning = false
		c.mu.Unlock()
		c.emitStatus()
		return err
	}

	c.mu.Lock()
	c.transitioning = false
	c.mu.Unlock()
	c.emitStatus()
	return nil
}

// startLocked does the heavy lifting outside the mutex (verify, hostlist,
// launch, DNS). It is only ever called with transitioning=true held, which
// serializes it against other Start/Stop calls.
func (c *Controller) startLocked(mode Mode, domains []string) error {
	// 1. Refuse to run tampered/missing binaries.
	if err := c.engine.Verify(engine.ModeFast); err != nil {
		return err
	}

	// 2. Resolve the hostlist for scoped modes.
	hostlistPath := ""
	if usesHostlist(mode) {
		var list []string
		switch mode {
		case ModeDiscord:
			list = discordDomains()
		case ModeCustom:
			list = normalizeDomains(domains)
			if len(list) == 0 {
				return fmt.Errorf("Custom Mode needs at least one valid domain")
			}
		}
		hostlistPath = filepath.Join(c.dataDir, "hostlist.txt")
		if err := writeHostlist(hostlistPath, list); err != nil {
			return err
		}
		c.mu.Lock()
		c.domains = normalizeDomains(list)
		c.mu.Unlock()
	} else {
		c.mu.Lock()
		c.domains = nil
		c.mu.Unlock()
	}

	// 3. Prepare the command line and process working directory.
	c.winwsPath = c.engine.WinwsPath()
	c.winwsDir = c.engine.Dir(engine.ModeFast)
	c.winwsArgs = buildArgs(hostlistPath)

	// 4. Launch winws and give the driver a moment to load; a crash-on-start
	//    (e.g. WinDivert refused) surfaces here *before* we touch DNS.
	p, err := c.launch()
	if err != nil {
		return c.classifyLaunchError(err, nil)
	}
	select {
	case <-p.exited:
		// Died during the grace window — almost certainly a load/config error.
		return c.classifyLaunchError(fmt.Errorf("winws exited immediately"), p.stderr)
	case <-time.After(minHealthyRuntime):
		// Still alive: consider the engine healthy.
	}

	// 5. Engine is up — now switch DNS to encrypted Cloudflare, recording the
	//    prior state to disk first so any teardown path can restore it.
	if err := c.dns.apply(context.Background()); err != nil {
		c.killCurrent()
		return fmt.Errorf("could not apply encrypted DNS: %w", err)
	}

	// 6. Promote to Running atomically. We hold the lock and re-check that the
	//    process is still alive: the supervisor cannot change state while we
	//    hold the lock, and it closes p.exited before it tries to, so this
	//    check is authoritative. If winws died during the DNS-apply window we
	//    must undo the DNS we just applied rather than report a false Running.
	c.mu.Lock()
	select {
	case <-p.exited:
		c.mu.Unlock()
		_ = c.dns.restore(context.Background())
		return c.classifyLaunchError(fmt.Errorf("winws exited during startup"), p.stderr)
	default:
	}
	c.dnsApplied = true
	c.state = StateRunning
	c.since = time.Now()
	c.mu.Unlock()
	if c.log != nil {
		c.log.Info("fast mode started", "mode", string(mode), "pid", p.cmd.Process.Pid, "domains", len(c.domains))
	}
	return nil
}

// launch starts one winws.exe process, wires up logging + stderr capture, and
// spawns its supervisor. The caller must have transitioning held (for the
// initial start) or be the supervisor itself (for a restart).
func (c *Controller) launch() (*proc, error) {
	cmd := exec.Command(c.winwsPath, c.winwsArgs...)
	cmd.Dir = c.winwsDir // so WinDivert.dll / .sys / cygwin1.dll resolve
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}

	ring := newRingBuffer(8 * 1024)
	// Tee winws output to the rotating winws.log and the in-memory ring
	// (used to explain crash-on-start failures to the user).
	out := &teeWriter{a: c.winwsLog, b: ring}
	cmd.Stdout = out
	cmd.Stderr = out

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	p := &proc{cmd: cmd, exited: make(chan struct{}), startedAt: time.Now(), stderr: ring}
	c.mu.Lock()
	c.cur = p
	c.mu.Unlock()

	go c.supervise(p)
	return p, nil
}

// supervise waits for a launched process to exit and decides what to do:
// nothing (an intentional Stop), restart (a transient drop), or give up and
// tear Fast Mode down (a persistent failure).
func (c *Controller) supervise(p *proc) {
	err := p.cmd.Wait()
	close(p.exited)
	ranFor := time.Since(p.startedAt)

	c.mu.Lock()
	// Stale supervisor (a newer launch or a Stop took over) — do nothing.
	if c.cur != p {
		c.mu.Unlock()
		return
	}
	// Still being started up: startLocked owns the outcome (it watches
	// p.exited during its grace/DNS-apply window). Don't auto-restart here —
	// that would orphan a process while startLocked reports failure. Clear
	// cur so startLocked's liveness check sees the death.
	if c.state == StateStarting {
		c.cur = nil
		c.mu.Unlock()
		return
	}
	// Intentional stop: Stop() owns the state transition and DNS restore.
	if c.state == StateStopping {
		c.mu.Unlock()
		return
	}

	if c.log != nil {
		c.log.Warn("winws exited unexpectedly", "ranFor", ranFor.String(), "error", fmt.Sprint(err))
	}

	if ranFor < minHealthyRuntime {
		c.fastFails++
	} else {
		c.fastFails = 0
	}
	c.restarts++

	giveUp := c.fastFails >= maxFastFails || c.restarts > maxRestarts
	if giveUp {
		c.lastErr = c.classifyLaunchError(fmt.Errorf("winws keeps exiting (%d attempts)", c.restarts), p.stderr).Error()
		c.state = StateFailed
		c.cur = nil
		c.mu.Unlock()
		if c.log != nil {
			c.log.Error("fast mode giving up after repeated winws failures", "restarts", c.restarts)
		}
		// Fast Mode isn't working — never leave the user on Cloudflare.
		c.teardownDNS()
		c.emitStatus()
		return
	}

	c.state = StateRestarting
	c.mu.Unlock()
	c.emitStatus()

	time.Sleep(restartBackoff)

	// Re-check we weren't stopped during the backoff.
	c.mu.Lock()
	if c.state != StateRestarting || c.cur != p {
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()

	np, err := c.launch()
	if err != nil {
		c.mu.Lock()
		c.lastErr = c.classifyLaunchError(err, nil).Error()
		c.state = StateFailed
		c.cur = nil
		c.mu.Unlock()
		c.teardownDNS()
		c.emitStatus()
		return
	}
	c.mu.Lock()
	if c.state == StateRestarting {
		c.state = StateRunning
	}
	c.mu.Unlock()
	_ = np
	c.emitStatus()
}

// Stop turns Fast Mode off: it kills winws and restores DNS. It is safe to
// call when already stopped (no-op) and always attempts the DNS restore even
// if killing the process fails.
func (c *Controller) Stop() error {
	c.mu.Lock()
	if c.state == StateStopped {
		c.mu.Unlock()
		// Even when we think we're stopped, honour any pending DNS backup —
		// belt and braces against a state/disk mismatch.
		return c.dns.restore(context.Background())
	}
	// Another transition (a Start, or a concurrent Stop) already owns the
	// lifecycle. Refuse rather than race it; the caller can retry. Shutdown
	// has its own unconditional DNS-restore backstop for the app-exit case.
	if c.transitioning {
		c.mu.Unlock()
		return fmt.Errorf("Fast Mode is busy changing state; try again in a moment")
	}
	c.transitioning = true
	c.state = StateStopping
	p := c.cur
	c.mu.Unlock()
	c.emitStatus()

	// Kill the process (if any) and wait for the supervisor to observe exit.
	if p != nil {
		c.killProc(p)
	}

	// Restore DNS no matter what — this is the guarantee.
	restoreErr := c.dns.restore(context.Background())

	c.mu.Lock()
	c.cur = nil
	c.dnsApplied = false
	c.state = StateStopped
	c.since = time.Time{}
	c.domains = nil
	if restoreErr != nil {
		c.lastErr = restoreErr.Error()
	} else {
		c.lastErr = ""
	}
	c.transitioning = false
	c.mu.Unlock()
	c.emitStatus()

	if c.log != nil {
		if restoreErr != nil {
			c.log.Error("fast mode stopped but DNS restore reported errors", "error", restoreErr)
		} else {
			c.log.Info("fast mode stopped and DNS restored")
		}
	}
	return restoreErr
}

// Shutdown is the app-exit hook: stop Fast Mode and restore DNS. Called from
// the Wails OnShutdown handler and as a final safety net in main. Because the
// user must never be left on Cloudflare, it always makes an unconditional DNS
// restore attempt at the end even if Stop reported busy or errored — restore
// is idempotent and a no-op when no backup exists.
func (c *Controller) Shutdown() {
	if err := c.Stop(); err != nil && c.log != nil {
		c.log.Error("fast mode shutdown stop error", "error", err)
	}
	if err := c.dns.restore(context.Background()); err != nil && c.log != nil {
		c.log.Error("fast mode shutdown DNS restore error", "error", err)
	}
	if c.winwsLog != nil {
		_ = c.winwsLog.Close()
	}
}

// killCurrent kills whatever process is current (used to roll back a failed
// start). It does not touch DNS or state.
func (c *Controller) killCurrent() {
	c.mu.Lock()
	p := c.cur
	c.cur = nil
	c.mu.Unlock()
	if p != nil {
		c.killProc(p)
	}
}

// killProc terminates a winws process and waits (bounded) for it to exit.
// It uses taskkill /T /F to reap any child processes, then falls back to a
// direct kill, so a wedged process can't survive teardown.
func (c *Controller) killProc(p *proc) {
	if p.cmd.Process != nil {
		pid := p.cmd.Process.Pid
		_, _ = runCommand(context.Background(), "taskkill", "/F", "/T", "/PID", fmt.Sprint(pid))
		_ = p.cmd.Process.Kill()
	}
	select {
	case <-p.exited:
	case <-time.After(killGrace):
		if c.log != nil {
			c.log.Error("winws did not exit within kill grace period", "pid", func() int {
				if p.cmd.Process != nil {
					return p.cmd.Process.Pid
				}
				return 0
			}())
		}
	}
}

// teardownDNS restores DNS and clears the applied flag (used on give-up).
func (c *Controller) teardownDNS() {
	if err := c.dns.restore(context.Background()); err != nil && c.log != nil {
		c.log.Error("DNS restore during teardown reported errors", "error", err)
	}
	c.mu.Lock()
	c.dnsApplied = false
	c.mu.Unlock()
}

// classifyLaunchError turns a raw launch/exit failure plus any captured winws
// output into a specific, actionable message. It recognises the common
// WinDivert driver-load and privilege failures.
func (c *Controller) classifyLaunchError(base error, stderr *ringBuffer) error {
	tail := ""
	if stderr != nil {
		tail = strings.ToLower(stderr.String())
	}
	switch {
	case strings.Contains(tail, "access is denied") || strings.Contains(tail, "error 5") || strings.Contains(tail, "code 5"):
		return fmt.Errorf("WinDivert was denied access — Fast Mode must run as Administrator. Restart Slipstream and approve the UAC prompt")
	case strings.Contains(tail, "signature") || strings.Contains(tail, "577") || strings.Contains(tail, "1275"):
		return fmt.Errorf("Windows blocked the WinDivert driver's signature. Ensure Secure Boot driver-signature enforcement isn't blocking it, then try again")
	case strings.Contains(tail, "windivert") && (strings.Contains(tail, "load") || strings.Contains(tail, "open") || strings.Contains(tail, "driver")):
		return fmt.Errorf("The WinDivert driver failed to load. Make sure no other DPI-bypass tool (another zapret, GoodbyeDPI, or a VPN's WinDivert) is running, then try again. Details: %s", firstLine(stderr))
	case strings.Contains(tail, "in use") || strings.Contains(tail, "already"):
		return fmt.Errorf("WinDivert is already in use by another program. Close other DPI-bypass tools or VPNs and try again")
	default:
		if s := firstLine(stderr); s != "" {
			return fmt.Errorf("%v: %s", base, s)
		}
		return fmt.Errorf("Fast Mode engine failed to start: %w", base)
	}
}

func firstLine(r *ringBuffer) string {
	if r == nil {
		return ""
	}
	s := strings.TrimSpace(r.String())
	if s == "" {
		return ""
	}
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// KillOrphanedProcesses is called once at start-up, before the UI comes up.
// A previous run that was hard-killed (task-managed, power loss) leaves its
// child winws.exe running: it has no job object tying its lifetime to ours,
// so terminating the parent doesn't terminate it. Best-effort; a missing
// process is not an error.
func KillOrphanedProcesses(log *slog.Logger) error {
	out, err := runCommand(context.Background(), "taskkill", "/IM", "winws.exe", "/F")
	if err != nil {
		lower := strings.ToLower(out)
		if strings.Contains(lower, "not found") {
			return nil
		}
		return fmt.Errorf("kill orphaned winws.exe: %w (%s)", err, strings.TrimSpace(out))
	}
	if log != nil {
		log.Warn("killed an orphaned winws.exe left over from a previous run")
	}
	return nil
}

// RecoverPendingDNS is called once at start-up, before the UI comes up. If a
// previous run left a DNS backup on disk (crash, hard kill, or power loss
// while Fast Mode was on), it restores the original DNS immediately so the
// user is never stranded on Cloudflare. A clean previous shutdown leaves no
// backup, making this a cheap no-op.
func RecoverPendingDNS(dataDir string, log *slog.Logger) error {
	dm := newDNSManager(filepath.Join(dataDir, "dns_backup.json"), log)
	if !dm.pending() {
		return nil
	}
	if log != nil {
		log.Warn("found pending DNS backup from a previous run; restoring original DNS")
	}
	return dm.restore(context.Background())
}
