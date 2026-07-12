package privatemode

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// traceEndpoint is Cloudflare's edge-diagnostics endpoint — the same trust
// boundary fastmode/dns.go already hardcodes for DNS. It returns a small
// plaintext body with an "ip=" line reporting the client's apparent address.
const traceEndpoint = "https://1.1.1.1/cdn-cgi/trace"

const externalIPTimeout = 4 * time.Second

// ExternalIP reports the address Private Mode's traffic currently appears to
// come from. It re-checks safety itself rather than trusting the caller's
// gating, since this is security-relevant: it must never make a request that
// could reveal anything while the tunnel isn't genuinely, currently up.
//
// With the kill switch armed, WFP already blocks any outbound packet that
// isn't through the tunnel adapter, the pinned VPS endpoint, or loopback —
// so a request that isn't actually routed through the tunnel can't leave the
// machine at all. The checks below exist to avoid even *trying* (and
// surfacing a stale or misleading answer) when the tunnel isn't up, not to
// enforce the no-leak guarantee themselves; that's the kill switch's job.
func (c *Controller) ExternalIP(ctx context.Context) (string, error) {
	c.mu.Lock()
	state := c.state
	lastHandshake := c.lastHandshake
	c.mu.Unlock()

	if state != StateConnected {
		return "", fmt.Errorf("Private Mode is not connected")
	}
	if !c.ks.IsArmed() {
		return "", fmt.Errorf("Kill switch is not armed")
	}
	if lastHandshake.IsZero() || time.Since(lastHandshake) >= handshakeFresh {
		return "", fmt.Errorf("Handshake is stale")
	}

	reqCtx, cancel := context.WithTimeout(ctx, externalIPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, traceEndpoint, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: externalIPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("external IP lookup failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("external IP lookup returned status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if ip, ok := strings.CutPrefix(line, "ip="); ok {
			ip = strings.TrimSpace(ip)
			if ip == "" {
				return "", fmt.Errorf("external IP lookup returned an empty address")
			}
			return ip, nil
		}
	}
	return "", fmt.Errorf("external IP lookup response did not include an address")
}
