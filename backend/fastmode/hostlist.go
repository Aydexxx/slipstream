package fastmode

import (
	"fmt"
	"os"
	"strings"
)

// writeHostlist writes the normalized, de-duplicated domains to path, one per
// line, in the format zapret's --hostlist expects. It returns an error if the
// resulting list is empty (a hostlist-scoped mode with no domains would
// silently de-censor nothing, which is almost certainly a caller bug).
func writeHostlist(path string, domains []string) error {
	clean := normalizeDomains(domains)
	if len(clean) == 0 {
		return fmt.Errorf("hostlist would be empty: no valid domains provided")
	}
	body := strings.Join(clean, "\r\n") + "\r\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write hostlist %s: %w", path, err)
	}
	return nil
}
