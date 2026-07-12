package killswitch

import (
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	iphlpapi                        = windows.NewLazySystemDLL("iphlpapi.dll")
	procConvertInterfaceAliasToLuid = iphlpapi.NewProc("ConvertInterfaceAliasToLuid")
)

// InterfaceLUID resolves a network interface's alias (friendly name, e.g. the
// tunnel adapter "Slipstream") to its NET_LUID, which the tunnel permit filter
// matches on IP_LOCAL_INTERFACE.
func InterfaceLUID(alias string) (uint64, error) {
	a, err := windows.UTF16PtrFromString(alias)
	if err != nil {
		return 0, err
	}
	var luid uint64
	r, _, _ := procConvertInterfaceAliasToLuid.Call(uintptr(unsafe.Pointer(a)), uintptr(unsafe.Pointer(&luid)))
	runtime.KeepAlive(a)
	if r != 0 {
		return 0, fmt.Errorf("ConvertInterfaceAliasToLuid(%q): error %d", alias, r)
	}
	return luid, nil
}
