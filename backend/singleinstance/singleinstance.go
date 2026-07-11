// Package singleinstance guards against multiple concurrent instances of
// Slipstream using a named Windows mutex.
package singleinstance

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// Guard holds the handle to the acquired named mutex.
type Guard struct {
	handle windows.Handle
}

// Acquire attempts to create the named global mutex. The returned bool is
// true if another instance already owns the mutex.
func Acquire(name string) (*Guard, bool, error) {
	namePtr, err := windows.UTF16PtrFromString(`Global\` + name)
	if err != nil {
		return nil, false, fmt.Errorf("encode mutex name: %w", err)
	}

	handle, err := windows.CreateMutex(nil, false, namePtr)
	if err != nil && err != windows.ERROR_ALREADY_EXISTS {
		return nil, false, fmt.Errorf("create mutex: %w", err)
	}

	return &Guard{handle: handle}, err == windows.ERROR_ALREADY_EXISTS, nil
}

// Release closes the mutex handle, freeing it for the next instance.
func (g *Guard) Release() {
	if g != nil && g.handle != 0 {
		windows.CloseHandle(g.handle)
	}
}

// NotifyAlreadyRunning shows a native message box informing the user that
// another instance is already running.
func NotifyAlreadyRunning(appName string) {
	text, err := windows.UTF16PtrFromString(appName + " is already running.")
	if err != nil {
		return
	}
	caption, err := windows.UTF16PtrFromString(appName)
	if err != nil {
		return
	}
	const mbOK = 0x00000000
	const mbIconInformation = 0x00000040
	_, _ = windows.MessageBox(windows.HWND(0), text, caption, mbOK|mbIconInformation)
}
