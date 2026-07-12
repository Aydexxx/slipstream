package privatemode

import (
	"fmt"
	"net"
	"strings"
)

// Config is a parsed AmneziaWG client configuration. Raw holds the exact
// original text (which is what we hand to amneziawg.exe /installtunnelservice —
// the obfuscation params must survive byte-for-byte), while the other fields
// are extracted for validation and for surfacing to the UI.
type Config struct {
	Raw string `json:"-"` // never serialized to the frontend (contains keys)

	InterfaceAddresses []string `json:"interfaceAddresses"`
	DNS                []string `json:"dns"`
	PeerPublicKey      string   `json:"peerPublicKey"`
	Endpoint           string   `json:"endpoint"` // host:port
	EndpointHost       string   `json:"endpointHost"`
	AllowedIPs         []string `json:"allowedIPs"`
	FullTunnel         bool     `json:"fullTunnel"` // AllowedIPs covers 0.0.0.0/0
	Obfuscated         bool     `json:"obfuscated"` // AmneziaWG Jc.. params present
	HasDNS             bool     `json:"hasDNS"`
}

// obfuscationKeys are the AmneziaWG-specific parameters (junk/header/decoy).
// Their presence distinguishes an AmneziaWG config from plain WireGuard.
var obfuscationKeys = map[string]bool{
	"jc": true, "jmin": true, "jmax": true,
	"s1": true, "s2": true, "s3": true, "s4": true,
	"h1": true, "h2": true, "h3": true, "h4": true,
	"i1": true, "i2": true, "i3": true, "i4": true, "i5": true,
}

// ParseConfig parses and validates an AmneziaWG client config. It is
// intentionally strict about the fields a *full-tunnel client* needs, so a
// bad import fails at import time with a clear message rather than at connect
// time as an opaque tunnel error.
func ParseConfig(raw string) (*Config, error) {
	c := &Config{Raw: raw}

	section := ""
	var (
		hasInterface, hasPeer bool
		privateKey, address   string
	)

	for i, rawLine := range strings.Split(raw, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			switch section {
			case "interface":
				hasInterface = true
			case "peer":
				hasPeer = true
			}
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("malformed line %d (expected key = value): %q", i+1, line)
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)

		if obfuscationKeys[key] {
			c.Obfuscated = true
		}

		switch section {
		case "interface":
			switch key {
			case "privatekey":
				privateKey = value
			case "address":
				address = value
				c.InterfaceAddresses = splitList(value)
			case "dns":
				c.DNS = splitList(value)
				c.HasDNS = len(c.DNS) > 0
			}
		case "peer":
			switch key {
			case "publickey":
				c.PeerPublicKey = value
			case "endpoint":
				c.Endpoint = value
				c.EndpointHost = endpointHost(value)
			case "allowedips":
				c.AllowedIPs = splitList(value)
			}
		}
	}

	// Validate the essentials for a working full-tunnel client.
	var missing []string
	if !hasInterface {
		missing = append(missing, "[Interface] section")
	}
	if !hasPeer {
		missing = append(missing, "[Peer] section")
	}
	if privateKey == "" {
		missing = append(missing, "[Interface] PrivateKey")
	}
	if address == "" {
		missing = append(missing, "[Interface] Address")
	}
	if c.PeerPublicKey == "" {
		missing = append(missing, "[Peer] PublicKey")
	}
	if c.Endpoint == "" {
		missing = append(missing, "[Peer] Endpoint")
	}
	if len(c.AllowedIPs) == 0 {
		missing = append(missing, "[Peer] AllowedIPs")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("invalid AmneziaWG config — missing: %s", strings.Join(missing, ", "))
	}

	for _, a := range c.AllowedIPs {
		if a == "0.0.0.0/0" || a == "::/0" {
			c.FullTunnel = true
		}
	}
	return c, nil
}

// Summary returns the UI-safe view of a config (no key material).
type Summary struct {
	Endpoint     string   `json:"endpoint"`
	EndpointHost string   `json:"endpointHost"`
	DNS          []string `json:"dns"`
	Addresses    []string `json:"addresses"`
	FullTunnel   bool     `json:"fullTunnel"`
	Obfuscated   bool     `json:"obfuscated"`
}

func (c *Config) Summary() Summary {
	return Summary{
		Endpoint:     c.Endpoint,
		EndpointHost: c.EndpointHost,
		DNS:          c.DNS,
		Addresses:    c.InterfaceAddresses,
		FullTunnel:   c.FullTunnel,
		Obfuscated:   c.Obfuscated,
	}
}

// ensureDNS guarantees the config sets a tunnel DNS, so name resolution goes
// through the VPS rather than leaking to (or being poisoned by) the local ISP
// resolver. If the [Interface] already has a DNS line, raw is returned
// unchanged; otherwise a Cloudflare default is injected right after the
// [Interface] header. Obfuscation params are left untouched.
func ensureDNS(raw, fallback string) string {
	// Already has DNS anywhere in the [Interface] section?
	section := ""
	for _, rawLine := range strings.Split(raw, "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		if section == "interface" {
			if key, _, ok := strings.Cut(line, "="); ok && strings.EqualFold(strings.TrimSpace(key), "dns") {
				return raw // already present
			}
		}
	}

	newline := "\n"
	if strings.Contains(raw, "\r\n") {
		newline = "\r\n"
	}
	out := make([]string, 0)
	injected := false
	for _, rawLine := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		out = append(out, rawLine)
		if !injected && strings.EqualFold(strings.TrimSpace(rawLine), "[Interface]") {
			out = append(out, "DNS = "+fallback)
			injected = true
		}
	}
	if !injected { // no [Interface] header found (shouldn't happen post-validate)
		return raw
	}
	return strings.Join(out, newline)
}

// pinEndpoint rewrites the [Peer] Endpoint host to a fixed IP (keeping the
// port), so the tunnel service connects by IP and never needs DNS — which the
// kill switch blocks. A no-op if the endpoint is already this IP.
func pinEndpoint(raw, ip string) string {
	newline := "\n"
	if strings.Contains(raw, "\r\n") {
		newline = "\r\n"
	}
	section := ""
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			section = strings.ToLower(strings.TrimSpace(t[1 : len(t)-1]))
			continue
		}
		if section != "peer" {
			continue
		}
		if key, val, ok := strings.Cut(t, "="); ok && strings.EqualFold(strings.TrimSpace(key), "endpoint") {
			if _, port, err := net.SplitHostPort(strings.TrimSpace(val)); err == nil {
				lines[i] = "Endpoint = " + net.JoinHostPort(ip, port)
			}
		}
	}
	return strings.Join(lines, newline)
}

func splitList(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// endpointHost returns the host portion of a "host:port" endpoint, tolerating
// bracketed IPv6 literals ("[2001:db8::1]:51820").
func endpointHost(endpoint string) string {
	e := strings.TrimSpace(endpoint)
	if strings.HasPrefix(e, "[") {
		if i := strings.Index(e, "]"); i >= 0 {
			return e[1:i]
		}
	}
	if i := strings.LastIndex(e, ":"); i >= 0 {
		return e[:i]
	}
	return e
}
