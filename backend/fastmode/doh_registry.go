package fastmode

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

// Windows 11 records per-interface DoH policy under the Dnscache service.
// Creating a Doh\<server-ip> subkey with DohFlags=1 tells the resolver to use
// encrypted DNS (auto-upgrade, plaintext fallback allowed) for that server on
// that interface. netsh registers the template system-wide; this makes the
// interface actually use it.
const (
	dohBaseKeyFmt = `SYSTEM\CurrentControlSet\Services\Dnscache\InterfaceSpecificParameters\%s\DohInterfaceSettings\Doh`
	dohFlagsAuto  = 1
)

// enableDoHForInterface writes the DoH policy for both Cloudflare IPs on the
// interface identified by guid (the "{....}" form). Best-effort: callers log
// and continue on error, since unencrypted-but-Cloudflare DNS is still a win.
func enableDoHForInterface(guid string) error {
	if guid == "" {
		return fmt.Errorf("empty interface GUID")
	}
	for _, server := range []string{cloudflarePrimary, cloudflareSecondary} {
		keyPath := fmt.Sprintf(dohBaseKeyFmt, guid) + `\` + server
		k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, keyPath, registry.SET_VALUE)
		if err != nil {
			return fmt.Errorf("create DoH key for %s: %w", server, err)
		}
		err = k.SetDWordValue("DohFlags", dohFlagsAuto)
		k.Close()
		if err != nil {
			return fmt.Errorf("set DohFlags for %s: %w", server, err)
		}
	}
	return nil
}

// removeDoHForInterface deletes the DoH policy keys we created for guid so a
// restored interface isn't left with a stale Cloudflare DoH override. Absent
// keys are treated as success.
func removeDoHForInterface(guid string) error {
	if guid == "" {
		return nil
	}
	base := fmt.Sprintf(dohBaseKeyFmt, guid)
	for _, server := range []string{cloudflarePrimary, cloudflareSecondary} {
		keyPath := base + `\` + server
		if err := registry.DeleteKey(registry.LOCAL_MACHINE, keyPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("delete DoH key for %s: %w", server, err)
		}
	}
	return nil
}
