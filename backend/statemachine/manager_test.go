package statemachine

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"slipstream/backend/fastmode"
)

// callLog records fast controller calls in an ordered sequence, so tests can
// assert the relative order of teardown vs. start.
type callLog struct {
	mu      sync.Mutex
	entries []string
}

func (c *callLog) add(s string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = append(c.entries, s)
}

func (c *callLog) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.entries))
	copy(out, c.entries)
	return out
}

type fakeFast struct {
	log *callLog

	mu         sync.Mutex
	startErr   error
	stopErr    error
	dnsPending bool
	status     fastmode.Status
	emitter    fastmode.Emitter

	// startGate, if non-nil, is closed by the test to release a blocked
	// Start() call — used to force overlap between concurrent Request calls.
	startGate chan struct{}
}

func (f *fakeFast) Start(mode fastmode.Mode, strategyID string, domains []string) error {
	f.log.add("fast:start")
	f.mu.Lock()
	gate := f.startGate
	err := f.startErr
	f.mu.Unlock()
	if gate != nil {
		<-gate
	}
	if err != nil {
		f.setStatus(fastmode.Status{State: fastmode.StateFailed, Error: err.Error()})
		return err
	}
	// Echo the requested strategy back in status, mirroring the real
	// controller, so the manager persists what actually ran.
	f.setStatus(fastmode.Status{State: fastmode.StateRunning, Mode: mode, Strategy: strategyID, Domains: domains})
	return nil
}

func (f *fakeFast) Stop() error {
	f.log.add("fast:stop")
	f.mu.Lock()
	err := f.stopErr
	f.mu.Unlock()
	f.setStatus(fastmode.Status{State: fastmode.StateStopped})
	return err
}

func (f *fakeFast) Status() fastmode.Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.status
}

func (f *fakeFast) SetEmitter(e fastmode.Emitter) {
	f.mu.Lock()
	f.emitter = e
	f.mu.Unlock()
}

func (f *fakeFast) DNSBackupPending() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.dnsPending
}

func (f *fakeFast) Shutdown() { f.log.add("fast:shutdown") }

func (f *fakeFast) Presets() map[string][]string             { return nil }
func (f *fakeFast) Strategies() []fastmode.StrategyInfo      { return nil }
func (f *fakeFast) LoadCustomDomains() ([]string, error)     { return nil, nil }
func (f *fakeFast) SaveCustomDomains(domains []string) error { return nil }

func (f *fakeFast) setStatus(s fastmode.Status) {
	f.mu.Lock()
	f.status = s
	e := f.emitter
	f.mu.Unlock()
	if e != nil {
		e(s)
	}
}

// statusRecorder captures every unified Status the Manager emits, so tests
// can assert on what the frontend would actually receive (in particular the
// transitioning flag driving the mode buttons' pending/disabled state).
type statusRecorder struct {
	mu       sync.Mutex
	statuses []Status
}

func (r *statusRecorder) emit(s Status) {
	r.mu.Lock()
	r.statuses = append(r.statuses, s)
	r.mu.Unlock()
}

func (r *statusRecorder) last() (Status, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.statuses) == 0 {
		return Status{}, false
	}
	return r.statuses[len(r.statuses)-1], true
}

func (r *statusRecorder) any(pred func(Status) bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.statuses {
		if pred(s) {
			return true
		}
	}
	return false
}

// harness bundles a Manager with its fake for a test.
type harness struct {
	mgr  *Manager
	fast *fakeFast
	log  *callLog
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	log := &callLog{}
	fast := &fakeFast{log: log}
	mgr, err := New(Config{
		Fast:         fast,
		StateDataDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return &harness{mgr: mgr, fast: fast, log: log}
}

func TestRequestFastModeActivates(t *testing.T) {
	h := newHarness(t)

	if err := h.mgr.RequestFastMode(fastmode.ModeFull, "", nil); err != nil {
		t.Fatalf("RequestFastMode: %v", err)
	}
	if got := h.mgr.Status(); got.State != StateFastActive || got.SubMode != SubModeFast {
		t.Fatalf("after fast start: state=%v subMode=%v", got.State, got.SubMode)
	}
	if indexOf(h.log.snapshot(), "fast:start") == -1 {
		t.Fatalf("RequestFastMode never reached fast:start, got %v", h.log.snapshot())
	}
}

func TestRequestIdleTeardownErrorGoesToError(t *testing.T) {
	h := newHarness(t)
	if err := h.mgr.RequestFastMode(fastmode.ModeFull, "", nil); err != nil {
		t.Fatalf("RequestFastMode: %v", err)
	}
	h.fast.mu.Lock()
	h.fast.stopErr = errors.New("dns restore failed")
	h.fast.mu.Unlock()

	err := h.mgr.RequestIdle()
	if err == nil {
		t.Fatal("expected RequestIdle to fail when teardown errors")
	}
	if got := h.mgr.Status(); got.State != StateError {
		t.Fatalf("expected StateError, got %v", got.State)
	}
}

func TestVerifyCleanBlocksIdleOnDirtyDNS(t *testing.T) {
	h := newHarness(t)
	if err := h.mgr.RequestFastMode(fastmode.ModeFull, "", nil); err != nil {
		t.Fatalf("RequestFastMode: %v", err)
	}
	// Stop() itself reports success, but the live DNS-pending signal says
	// otherwise — verifyClean must still block returning to idle.
	h.fast.mu.Lock()
	h.fast.dnsPending = true
	h.fast.mu.Unlock()

	err := h.mgr.RequestIdle()
	if err == nil || !strings.Contains(err.Error(), "DNS") {
		t.Fatalf("expected a DNS-pending teardown error, got %v", err)
	}
	if got := h.mgr.Status(); got.State != StateError {
		t.Fatalf("expected StateError, got %v", got.State)
	}
}

// TestRequestFastModeEmitsSettledStatus guards the BUG 1 regression: the final
// status emitted once a transition finishes must carry transitioning=false.
// The last emit inside RequestFastMode snapshots transitioning=true (the lock
// is still held), so without endTransition's own emit the frontend would never
// see it clear and the mode buttons would stay stuck in their loading state.
func TestRequestFastModeEmitsSettledStatus(t *testing.T) {
	h := newHarness(t)
	rec := &statusRecorder{}
	h.mgr.SetEmitter(rec.emit)

	if err := h.mgr.RequestFastMode(fastmode.ModeFull, "", nil); err != nil {
		t.Fatalf("RequestFastMode: %v", err)
	}

	last, ok := rec.last()
	if !ok {
		t.Fatal("expected at least one emitted status")
	}
	if last.Transitioning {
		t.Fatalf("final emitted status still transitioning=true: %+v", last)
	}
	if last.State != StateFastActive || last.SubMode != SubModeFast {
		t.Fatalf("final emitted status not fast-active: %+v", last)
	}
}

// TestRequestIdleStopsFastAndClearsPending is the end-to-end stop path behind
// the "Turn Off" button: it must reach the controller's Stop, signal a pending
// transition, and emit a settled idle status so the spinner clears.
func TestRequestIdleStopsFastAndClearsPending(t *testing.T) {
	h := newHarness(t)
	rec := &statusRecorder{}
	h.mgr.SetEmitter(rec.emit)

	if err := h.mgr.RequestFastMode(fastmode.ModeFull, "", nil); err != nil {
		t.Fatalf("RequestFastMode: %v", err)
	}
	if err := h.mgr.RequestIdle(); err != nil {
		t.Fatalf("RequestIdle: %v", err)
	}

	if indexOf(h.log.snapshot(), "fast:stop") == -1 {
		t.Fatalf("RequestIdle never reached fast:stop, got %v", h.log.snapshot())
	}
	// A pending state was signalled (begin emits transitioning=true) so the UI
	// can show progress for the whole round-trip...
	if !rec.any(func(s Status) bool { return s.Transitioning }) {
		t.Fatal("expected at least one emitted status with transitioning=true")
	}
	// ...and the final emit settles it so the pending UI resolves.
	last, _ := rec.last()
	if last.Transitioning || last.State != StateIdle || last.SubMode != SubModeNone {
		t.Fatalf("final emitted status not settled-idle: %+v", last)
	}
	if got := h.mgr.Status(); got.State != StateIdle || got.SubMode != SubModeNone || got.Transitioning {
		t.Fatalf("final Status not idle: %+v", got)
	}
}

// TestRequestIdleWhenAlreadyIdleIsClean confirms turning off from a clean
// idle state is a harmless no-op that still settles with transitioning=false.
func TestRequestIdleWhenAlreadyIdleIsClean(t *testing.T) {
	h := newHarness(t)
	rec := &statusRecorder{}
	h.mgr.SetEmitter(rec.emit)

	if err := h.mgr.RequestIdle(); err != nil {
		t.Fatalf("RequestIdle: %v", err)
	}
	for _, e := range h.log.snapshot() {
		if e == "fast:stop" {
			t.Fatalf("no teardown should run when already idle, got %v", h.log.snapshot())
		}
	}
	last, ok := rec.last()
	if !ok {
		t.Fatal("expected at least one emitted status")
	}
	if last.Transitioning || last.State != StateIdle || last.SubMode != SubModeNone {
		t.Fatalf("final emitted status not settled-idle: %+v", last)
	}
}

func TestConcurrentRequestsSerialize(t *testing.T) {
	h := newHarness(t)
	h.fast.mu.Lock()
	h.fast.startGate = make(chan struct{})
	gate := h.fast.startGate
	h.fast.mu.Unlock()

	errCh := make(chan error, 1)
	go func() {
		errCh <- h.mgr.RequestFastMode(fastmode.ModeFull, "", nil)
	}()

	// Give the first call time to claim the transition lock and block inside
	// fast:start.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if idx := indexOf(h.log.snapshot(), "fast:start"); idx != -1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("first RequestFastMode never reached fast:start")
		}
		time.Sleep(time.Millisecond)
	}

	second := h.mgr.RequestIdle()
	if second == nil || !strings.Contains(second.Error(), "changing mode") {
		t.Fatalf("expected the second call to be rejected as busy, got %v", second)
	}

	close(gate)
	if err := <-errCh; err != nil {
		t.Fatalf("first RequestFastMode: %v", err)
	}
	for _, e := range h.log.snapshot() {
		if e == "fast:stop" {
			t.Fatal("fast:stop must never run while fast mode was starting")
		}
	}
}

func TestAsyncFailedFlipsToErrorOutsideRequestCall(t *testing.T) {
	h := newHarness(t)
	if err := h.mgr.RequestFastMode(fastmode.ModeFull, "", nil); err != nil {
		t.Fatalf("RequestFastMode: %v", err)
	}

	// Simulate the supervisor firing asynchronously (e.g. winws.exe crashed
	// out for good), with no Request* call in flight.
	h.fast.setStatus(fastmode.Status{State: fastmode.StateFailed, Error: "winws exited repeatedly"})

	got := h.mgr.Status()
	if got.State != StateError {
		t.Fatalf("expected StateError after async Failed, got %v", got.State)
	}
	if got.Error != "winws exited repeatedly" {
		t.Fatalf("expected the failure message to surface, got %q", got.Error)
	}
}

func TestSettingsPersistAcrossManagerInstances(t *testing.T) {
	dir := t.TempDir()
	log1 := &callLog{}
	fast1 := &fakeFast{log: log1}
	mgr1, err := New(Config{Fast: fast1, StateDataDir: dir})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := mgr1.RequestFastMode(fastmode.ModeDiscord, "", []string{"example.com"}); err != nil {
		t.Fatalf("RequestFastMode: %v", err)
	}
	if err := mgr1.SetReconnectOnLaunch(true); err != nil {
		t.Fatalf("SetReconnectOnLaunch: %v", err)
	}

	log2 := &callLog{}
	fast2 := &fakeFast{log: log2}
	mgr2, err := New(Config{Fast: fast2, StateDataDir: dir})
	if err != nil {
		t.Fatalf("second New: %v", err)
	}
	mgr2.mu.Lock()
	s := mgr2.settings
	mgr2.mu.Unlock()
	if s.LastMode != SubModeFast || s.LastFastSubMode != string(fastmode.ModeDiscord) || !s.ReconnectOnLaunch {
		t.Fatalf("settings did not round-trip: %+v", s)
	}
	if len(s.LastFastDomains) != 1 || s.LastFastDomains[0] != "example.com" {
		t.Fatalf("domains did not round-trip: %+v", s.LastFastDomains)
	}
}

func TestShutdownCalledTwiceOnlyTearsDownOnce(t *testing.T) {
	h := newHarness(t)
	if err := h.mgr.RequestFastMode(fastmode.ModeFull, "", nil); err != nil {
		t.Fatalf("RequestFastMode: %v", err)
	}

	h.mgr.Shutdown()
	h.mgr.Shutdown()

	entries := h.log.snapshot()
	count := 0
	for _, e := range entries {
		if e == "fast:shutdown" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected fast:shutdown exactly once, got %d in %v", count, entries)
	}
}

func TestLastFastSelectionDefaultsToFull(t *testing.T) {
	h := newHarness(t)
	mode, strategy, domains := h.mgr.LastFastSelection()
	if mode != fastmode.ModeFull {
		t.Errorf("expected default mode ModeFull, got %q", mode)
	}
	if strategy != "" {
		t.Errorf("expected empty strategy by default (resolved downstream), got %q", strategy)
	}
	if len(domains) != 0 {
		t.Errorf("expected no domains by default, got %v", domains)
	}
}

func TestLastFastSelectionReflectsMostRecentStart(t *testing.T) {
	h := newHarness(t)
	if err := h.mgr.RequestFastMode(fastmode.ModeDiscord, "vodafone", []string{"example.com"}); err != nil {
		t.Fatalf("RequestFastMode: %v", err)
	}
	mode, strategy, domains := h.mgr.LastFastSelection()
	if mode != fastmode.ModeDiscord {
		t.Errorf("expected ModeDiscord, got %q", mode)
	}
	if strategy != "vodafone" {
		t.Errorf("expected strategy vodafone, got %q", strategy)
	}
	if len(domains) != 1 || domains[0] != "example.com" {
		t.Errorf("expected [example.com], got %v", domains)
	}
}

func indexOf(entries []string, target string) int {
	for i, e := range entries {
		if e == target {
			return i
		}
	}
	return -1
}
