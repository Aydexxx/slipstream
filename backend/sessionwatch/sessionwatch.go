// Package sessionwatch catches Windows logging off or shutting down while
// Slipstream is running, so DNS/routes/WFP filters are torn down before the
// OS kills the process rather than being left dirty until the next launch's
// crash recovery.
//
// Wails has no hook for this (WM_QUERYENDSESSION/WM_ENDSESSION handling
// lives entirely in its internal, unexported window code), so this package
// runs its own tiny hidden message-only window with its own message loop —
// the standard, documented way to observe session-end independent of
// whatever other windows a process owns.
package sessionwatch

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32 = syscall.NewLazyDLL("user32.dll")

	procRegisterClassExW           = user32.NewProc("RegisterClassExW")
	procUnregisterClassW           = user32.NewProc("UnregisterClassW")
	procCreateWindowExW            = user32.NewProc("CreateWindowExW")
	procDestroyWindow              = user32.NewProc("DestroyWindow")
	procDefWindowProcW             = user32.NewProc("DefWindowProcW")
	procGetMessageW                = user32.NewProc("GetMessageW")
	procTranslateMessage           = user32.NewProc("TranslateMessage")
	procDispatchMessageW           = user32.NewProc("DispatchMessageW")
	procPostMessageW               = user32.NewProc("PostMessageW")
	procPostQuitMessage            = user32.NewProc("PostQuitMessage")
	procShutdownBlockReasonCreate  = user32.NewProc("ShutdownBlockReasonCreate")
	procShutdownBlockReasonDestroy = user32.NewProc("ShutdownBlockReasonDestroy")
)

const (
	wmDestroy         = 0x0002
	wmClose           = 0x0010
	wmQuit            = 0x0012
	wmQueryEndSession = 0x0011

	// hwndMessage is HWND_MESSAGE — creating a window with this as its
	// parent makes it message-only: no UI, no taskbar presence, but it
	// still receives broadcast messages like WM_QUERYENDSESSION.
	hwndMessage = ^uintptr(2) // (HWND)(-3)

	className = "SlipstreamSessionWatcher"
)

// teardownTimeout bounds how long we let onEnd run before giving up and
// returning control to Windows anyway — a hung teardown must not be able to
// hang the whole OS shutdown indefinitely. A var (not const) so tests can
// shrink it rather than waiting out the real value.
var teardownTimeout = 15 * time.Second

type wndClassExW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     syscall.Handle
	hIcon         syscall.Handle
	hCursor       syscall.Handle
	hbrBackground syscall.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       syscall.Handle
}

type point struct{ x, y int32 }

type msg struct {
	hwnd    syscall.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

// Watcher owns the hidden window and message loop.
type Watcher struct {
	hwnd     syscall.Handle
	class    *uint16
	onEnd    func()
	doneOnce sync.Once
	stopped  chan struct{}
}

// Watch starts watching for Windows session-end (logoff/shutdown) on a
// dedicated, OS-thread-locked goroutine. onEnd is called synchronously,
// bounded by an internal timeout, when Windows asks to end the session; it
// should perform whatever teardown must complete before the OS proceeds.
// Call the returned stop func to tear the watcher down on a normal exit.
func Watch(onEnd func()) (stop func(), err error) {
	w := &Watcher{onEnd: onEnd, stopped: make(chan struct{})}

	ready := make(chan error, 1)
	go w.run(ready)

	if err := <-ready; err != nil {
		return func() {}, err
	}
	return w.stop, nil
}

func (w *Watcher) run(ready chan<- error) {
	// A Win32 window and its message loop must live on one fixed OS thread
	// for their entire lifetime.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	classNamePtr, err := syscall.UTF16PtrFromString(className)
	if err != nil {
		ready <- err
		return
	}
	w.class = classNamePtr

	wc := wndClassExW{
		lpfnWndProc:   syscall.NewCallback(w.wndProc),
		lpszClassName: classNamePtr,
	}
	wc.cbSize = uint32(unsafe.Sizeof(wc))

	if r, _, callErr := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		ready <- fmt.Errorf("sessionwatch: RegisterClassExW: %w", callErr)
		return
	}

	hwnd, _, callErr := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(classNamePtr)),
		uintptr(unsafe.Pointer(classNamePtr)),
		0, 0, 0, 0, 0,
		hwndMessage,
		0, 0, 0,
	)
	if hwnd == 0 {
		procUnregisterClassW.Call(uintptr(unsafe.Pointer(classNamePtr)))
		ready <- fmt.Errorf("sessionwatch: CreateWindowExW: %w", callErr)
		return
	}
	w.hwnd = syscall.Handle(hwnd)

	ready <- nil

	var m msg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		// GetMessage returns 0 on WM_QUIT, -1 (as a large uintptr) on error.
		if r == 0 || r == ^uintptr(0) {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}

	procUnregisterClassW.Call(uintptr(unsafe.Pointer(classNamePtr)))
	close(w.stopped)
}

func (w *Watcher) wndProc(hwnd, message, wParam, lParam uintptr) uintptr {
	switch uint32(message) {
	case wmQueryEndSession:
		w.runTeardown()
		return 1 // allow the session to end

	case wmClose, wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}

	r, _, _ := procDefWindowProcW.Call(hwnd, message, wParam, lParam)
	return r
}

// runTeardown asks Windows to hold off force-killing us while onEnd runs,
// bounded by teardownTimeout so a hang here can't hang the OS shutdown.
// Guarded by doneOnce since WM_QUERYENDSESSION can in principle be delivered
// more than once before the process actually exits.
func (w *Watcher) runTeardown() {
	w.doneOnce.Do(func() {
		if w.onEnd == nil {
			return
		}
		reason, _ := syscall.UTF16PtrFromString("Slipstream is restoring network settings…")
		procShutdownBlockReasonCreate.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(reason)))
		defer procShutdownBlockReasonDestroy.Call(uintptr(w.hwnd))

		done := make(chan struct{})
		go func() {
			w.onEnd()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(teardownTimeout):
		}
	})
}

// stop tears the watcher down for a normal (non-session-end) exit: posts
// WM_CLOSE to the message loop and waits for it to unwind.
func (w *Watcher) stop() {
	procPostMessageW.Call(uintptr(w.hwnd), wmClose, 0, 0)
	<-w.stopped
}
