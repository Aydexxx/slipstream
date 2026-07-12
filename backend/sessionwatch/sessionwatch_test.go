package sessionwatch

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestWatchCreatesAndStopsCleanly(t *testing.T) {
	stop, err := Watch(func() {})
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}

	done := make(chan struct{})
	go func() {
		stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("stop() did not return in time")
	}
}

// wndProc is exercised directly (white-box, same package) rather than via a
// real SendMessage — this tests the exact dispatch/teardown logic without
// depending on OS message delivery timing.
func TestWndProcQueryEndSessionRunsOnEndOnceAndReturnsTrue(t *testing.T) {
	var calls int32
	w := &Watcher{
		onEnd:   func() { atomic.AddInt32(&calls, 1) },
		stopped: make(chan struct{}),
	}

	if r := w.wndProc(0, wmQueryEndSession, 0, 0); r != 1 {
		t.Errorf("first WM_QUERYENDSESSION: got %d, want 1 (allow)", r)
	}
	// A second delivery (Windows can in principle send it more than once)
	// must not re-run teardown.
	if r := w.wndProc(0, wmQueryEndSession, 0, 0); r != 1 {
		t.Errorf("second WM_QUERYENDSESSION: got %d, want 1", r)
	}

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("onEnd called %d times, want 1", got)
	}
}

func TestWndProcQueryEndSessionBoundedByTimeout(t *testing.T) {
	orig := teardownTimeout
	teardownTimeout = 50 * time.Millisecond
	defer func() { teardownTimeout = orig }()

	hang := make(chan struct{})
	defer close(hang)
	w := &Watcher{
		onEnd:   func() { <-hang }, // never returns on its own
		stopped: make(chan struct{}),
	}

	done := make(chan struct{})
	go func() {
		w.wndProc(0, wmQueryEndSession, 0, 0)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WM_QUERYENDSESSION handling did not return within the timeout bound")
	}
}

func TestWndProcCloseIsHandled(t *testing.T) {
	w := &Watcher{stopped: make(chan struct{})}
	if r := w.wndProc(0, wmClose, 0, 0); r != 0 {
		t.Errorf("WM_CLOSE: got %d, want 0 (handled)", r)
	}
}
