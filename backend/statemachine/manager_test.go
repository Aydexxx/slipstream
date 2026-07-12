package statemachine

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"slipstream/backend/fastmode"
	"slipstream/backend/privatemode"
)

// callLog records fast/private controller calls in a single ordered
// sequence, shared between both fakes, so tests can assert the relative
// order of teardown vs. start across the two controllers.
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

func (f *fakeFast) Start(mode fastmode.Mode, domains []string) error {
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
	f.setStatus(fastmode.Status{State: fastmode.StateRunning, Mode: mode, Domains: domains})
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

type fakePrivate struct {
	log *callLog

	mu              sync.Mutex
	connectErr      error
	disconnectErr   error
	killSwitchArmed bool
	status          privatemode.Status
	emitter         privatemode.Emitter
}

func (p *fakePrivate) Connect() error {
	p.log.add("private:connect")
	p.mu.Lock()
	err := p.connectErr
	p.mu.Unlock()
	if err != nil {
		p.setStatus(privatemode.Status{State: privatemode.StateError, Error: err.Error()})
		return err
	}
	p.setStatus(privatemode.Status{State: privatemode.StateConnecting})
	return nil
}

func (p *fakePrivate) Disconnect() error {
	p.log.add("private:disconnect")
	p.mu.Lock()
	err := p.disconnectErr
	p.mu.Unlock()
	p.setStatus(privatemode.Status{State: privatemode.StateDisconnected})
	return err
}

func (p *fakePrivate) Status() privatemode.Status {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

func (p *fakePrivate) SetEmitter(e privatemode.Emitter) {
	p.mu.Lock()
	p.emitter = e
	p.mu.Unlock()
}

func (p *fakePrivate) KillSwitchArmed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.killSwitchArmed
}

func (p *fakePrivate) DisarmKillSwitch() error {
	p.log.add("private:disarm")
	p.mu.Lock()
	p.killSwitchArmed = false
	p.mu.Unlock()
	return nil
}

func (p *fakePrivate) Shutdown() { p.log.add("private:shutdown") }

func (p *fakePrivate) ImportConfig(raw string) (privatemode.Summary, error) {
	return privatemode.Summary{}, nil
}
func (p *fakePrivate) HasConfig() bool                             { return false }
func (p *fakePrivate) ConfigSummary() (privatemode.Summary, error) { return privatemode.Summary{}, nil }
func (p *fakePrivate) DeleteConfig() error                         { return nil }
func (p *fakePrivate) ExternalIP(ctx context.Context) (string, error) {
	return "", errors.New("not implemented in fake")
}

func (p *fakePrivate) setStatus(s privatemode.Status) {
	p.mu.Lock()
	p.status = s
	e := p.emitter
	p.mu.Unlock()
	if e != nil {
		e(s)
	}
}

// harness bundles a Manager with its fakes for a test.
type harness struct {
	mgr     *Manager
	fast    *fakeFast
	private *fakePrivate
	log     *callLog
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	log := &callLog{}
	fast := &fakeFast{log: log}
	private := &fakePrivate{log: log}
	mgr, err := New(Config{
		Fast:         fast,
		Private:      private,
		StateDataDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return &harness{mgr: mgr, fast: fast, private: private, log: log}
}

func TestSwitchFastToPrivateTearsDownFastFirstAndVerifies(t *testing.T) {
	h := newHarness(t)

	if err := h.mgr.RequestFastMode(fastmode.ModeFull, nil); err != nil {
		t.Fatalf("RequestFastMode: %v", err)
	}
	if got := h.mgr.Status(); got.State != StateFastActive || got.SubMode != SubModeFast {
		t.Fatalf("after fast start: state=%v subMode=%v", got.State, got.SubMode)
	}

	if err := h.mgr.RequestPrivateMode(); err != nil {
		t.Fatalf("RequestPrivateMode: %v", err)
	}
	if got := h.mgr.Status(); got.State != StatePrivateConnecting || got.SubMode != SubModePrivate {
		t.Fatalf("after private connect: state=%v subMode=%v", got.State, got.SubMode)
	}

	entries := h.log.snapshot()
	stopIdx, connectIdx := indexOf(entries, "fast:stop"), indexOf(entries, "private:connect")
	if stopIdx == -1 || connectIdx == -1 || stopIdx > connectIdx {
		t.Fatalf("expected fast:stop before private:connect, got %v", entries)
	}
}

func TestTeardownErrorBlocksSwitchAndDoesNotStartTarget(t *testing.T) {
	h := newHarness(t)
	if err := h.mgr.RequestFastMode(fastmode.ModeFull, nil); err != nil {
		t.Fatalf("RequestFastMode: %v", err)
	}
	h.fast.mu.Lock()
	h.fast.stopErr = errors.New("dns restore failed")
	h.fast.mu.Unlock()

	err := h.mgr.RequestPrivateMode()
	if err == nil {
		t.Fatal("expected RequestPrivateMode to fail when teardown errors")
	}
	if got := h.mgr.Status(); got.State != StateError {
		t.Fatalf("expected StateError, got %v", got.State)
	}
	for _, e := range h.log.snapshot() {
		if e == "private:connect" {
			t.Fatal("private:connect must not be called when fast teardown failed")
		}
	}
}

func TestVerifyCleanBlocksSwitchOnDirtyDNS(t *testing.T) {
	h := newHarness(t)
	if err := h.mgr.RequestFastMode(fastmode.ModeFull, nil); err != nil {
		t.Fatalf("RequestFastMode: %v", err)
	}
	// Stop() itself reports success, but the live DNS-pending signal says
	// otherwise — verifyClean must still block the switch.
	h.fast.mu.Lock()
	h.fast.dnsPending = true
	h.fast.mu.Unlock()

	err := h.mgr.RequestPrivateMode()
	if err == nil || !strings.Contains(err.Error(), "DNS") {
		t.Fatalf("expected a DNS-pending teardown error, got %v", err)
	}
	for _, e := range h.log.snapshot() {
		if e == "private:connect" {
			t.Fatal("private:connect must not be called when DNS is still pending restore")
		}
	}
}

func TestVerifyCleanBlocksSwitchOnArmedKillSwitch(t *testing.T) {
	h := newHarness(t)
	if err := h.mgr.RequestPrivateMode(); err != nil {
		t.Fatalf("RequestPrivateMode: %v", err)
	}
	// Disconnect() itself reports success, but the live kill-switch-armed
	// signal says otherwise — verifyClean must still block the switch.
	h.private.mu.Lock()
	h.private.killSwitchArmed = true
	h.private.mu.Unlock()

	err := h.mgr.RequestFastMode(fastmode.ModeFull, nil)
	if err == nil || !strings.Contains(err.Error(), "kill switch") {
		t.Fatalf("expected a kill-switch-armed teardown error, got %v", err)
	}
	for _, e := range h.log.snapshot() {
		if e == "fast:start" {
			t.Fatal("fast:start must not be called when the kill switch is still armed")
		}
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
		errCh <- h.mgr.RequestFastMode(fastmode.ModeFull, nil)
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

	second := h.mgr.RequestPrivateMode()
	if second == nil || !strings.Contains(second.Error(), "changing mode") {
		t.Fatalf("expected the second call to be rejected as busy, got %v", second)
	}

	close(gate)
	if err := <-errCh; err != nil {
		t.Fatalf("first RequestFastMode: %v", err)
	}
	for _, e := range h.log.snapshot() {
		if e == "private:connect" {
			t.Fatal("private:connect must never run while fast mode was starting")
		}
	}
}

func TestAsyncNoHandshakeFlipsToErrorOutsideRequestCall(t *testing.T) {
	h := newHarness(t)
	if err := h.mgr.RequestPrivateMode(); err != nil {
		t.Fatalf("RequestPrivateMode: %v", err)
	}

	// Simulate the monitor goroutine's giveUp() firing asynchronously, with
	// no Request* call in flight.
	h.private.setStatus(privatemode.Status{State: privatemode.StateNoHandshake, Error: "handshake never completed"})

	got := h.mgr.Status()
	if got.State != StateError {
		t.Fatalf("expected StateError after async NoHandshake, got %v", got.State)
	}
	if got.Error != "handshake never completed" {
		t.Fatalf("expected the turkey message to surface, got %q", got.Error)
	}
}

func TestSettingsPersistAcrossManagerInstances(t *testing.T) {
	dir := t.TempDir()
	log1 := &callLog{}
	fast1 := &fakeFast{log: log1}
	private1 := &fakePrivate{log: log1}
	mgr1, err := New(Config{Fast: fast1, Private: private1, StateDataDir: dir})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := mgr1.RequestFastMode(fastmode.ModeDiscord, []string{"example.com"}); err != nil {
		t.Fatalf("RequestFastMode: %v", err)
	}
	if err := mgr1.SetReconnectOnLaunch(true); err != nil {
		t.Fatalf("SetReconnectOnLaunch: %v", err)
	}

	log2 := &callLog{}
	fast2 := &fakeFast{log: log2}
	private2 := &fakePrivate{log: log2}
	mgr2, err := New(Config{Fast: fast2, Private: private2, StateDataDir: dir})
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
	if err := h.mgr.RequestFastMode(fastmode.ModeFull, nil); err != nil {
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
	mode, domains := h.mgr.LastFastSelection()
	if mode != fastmode.ModeFull {
		t.Errorf("expected default mode ModeFull, got %q", mode)
	}
	if len(domains) != 0 {
		t.Errorf("expected no domains by default, got %v", domains)
	}
}

func TestLastFastSelectionReflectsMostRecentStart(t *testing.T) {
	h := newHarness(t)
	if err := h.mgr.RequestFastMode(fastmode.ModeDiscord, []string{"example.com"}); err != nil {
		t.Fatalf("RequestFastMode: %v", err)
	}
	mode, domains := h.mgr.LastFastSelection()
	if mode != fastmode.ModeDiscord {
		t.Errorf("expected ModeDiscord, got %q", mode)
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
