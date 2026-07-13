// Package comsta initialises a single-threaded COM apartment (STA) on the
// current goroutine's locked OS thread.
//
// Microsoft Edge WebView2 requires the thread that creates its environment to
// belong to a COM STA. Wails' go-webview2 dependency arranges this in a package
// init() — but only on the process's *main* OS thread. Slipstream reserves the
// main (original) goroutine for getlantern/systray, which likewise pins it in
// its own init(), so wails.Run is driven from a separate goroutine whose OS
// thread has no COM apartment. Without the apartment set up here, WebView2
// environment creation fails with "CoInitialize has not been called" and the
// window never appears.
//
// This is intentionally isolated to the UI goroutine that calls Init: it does
// not touch the main thread's apartment or sessionwatch's dedicated thread.
package comsta

import (
	"fmt"
	"runtime"
	"syscall"
)

var (
	ole32              = syscall.NewLazyDLL("ole32.dll")
	procCoInitializeEx = ole32.NewProc("CoInitializeEx")
	procCoUninitialize = ole32.NewProc("CoUninitialize")
)

const (
	// coinitApartmentThreaded selects an STA — the same mode go-webview2 uses
	// on the main thread, so both threads agree on apartment type.
	coinitApartmentThreaded = 0x2

	// CoInitializeEx HRESULTs we care about. S_OK and S_FALSE both leave the
	// thread in an STA (S_FALSE just means it was already initialised, and
	// still requires a matching CoUninitialize). RPC_E_CHANGED_MODE means the
	// thread is already in a *different* (MTA) apartment, which WebView2 can't
	// use — surface that rather than silently proceeding.
	sOK             = 0x00000000
	sFALSE          = 0x00000001
	rpcEChangedMode = 0x80010106
)

// Init locks the calling goroutine to its OS thread and enters a COM STA on it.
// Call the returned func (typically via defer) once the thread's UI work is
// finished to leave the apartment and release the thread.
func Init() (uninit func(), err error) {
	runtime.LockOSThread()

	hr, _, _ := procCoInitializeEx.Call(0, coinitApartmentThreaded)
	switch uint32(hr) {
	case sOK, sFALSE:
		// Apartment ready; balance with one CoUninitialize on the way out.
		return func() {
			procCoUninitialize.Call()
			runtime.UnlockOSThread()
		}, nil
	case rpcEChangedMode:
		runtime.UnlockOSThread()
		return func() {}, fmt.Errorf("comsta: thread already in a non-STA apartment (hr=0x%08x)", uint32(hr))
	default:
		runtime.UnlockOSThread()
		return func() {}, fmt.Errorf("comsta: CoInitializeEx failed (hr=0x%08x)", uint32(hr))
	}
}
