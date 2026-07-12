package fastmode

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// loadCustomDomains reads the user's persisted custom domain list from path.
// A missing file is not an error — it just means the user hasn't saved one
// yet, so an empty slice is returned. Blank lines and lines beginning with
// '#' (comments) are ignored, and every entry is normalized.
func loadCustomDomains(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("open custom domain list: %w", err)
	}
	defer f.Close()

	var raw []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		raw = append(raw, line)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read custom domain list: %w", err)
	}
	return normalizeDomains(raw), nil
}

// saveCustomDomains persists the user's custom domain list to path,
// normalized and de-duplicated, one domain per line. Writing is atomic
// (temp file + rename) so a crash mid-write can't corrupt the saved list.
func saveCustomDomains(path string, domains []string) error {
	clean := normalizeDomains(domains)
	body := "# Slipstream Fast Mode - custom domains (one per line)\r\n" +
		strings.Join(clean, "\r\n")
	if len(clean) > 0 {
		body += "\r\n"
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write custom domain list: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("save custom domain list: %w", err)
	}
	return nil
}
