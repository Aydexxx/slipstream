package fastmode

// Mode is a Fast Mode sub-mode selected by the user.
type Mode string

const (
	// ModeFull de-censors all HTTPS (TCP/443) and QUIC (UDP/443) traffic,
	// regardless of destination host. No hostlist.
	ModeFull Mode = "full"
	// ModeDiscord de-censors only Discord's domains.
	ModeDiscord Mode = "discord"
	// ModeCustom de-censors only the domains the user selected/entered.
	ModeCustom Mode = "custom"
)

// validMode reports whether m is a recognised sub-mode.
func validMode(m Mode) bool {
	switch m {
	case ModeFull, ModeDiscord, ModeCustom:
		return true
	default:
		return false
	}
}

// usesHostlist reports whether a sub-mode scopes desync to a hostlist file.
// Full mode applies to every 443 connection and therefore has no hostlist.
func usesHostlist(m Mode) bool {
	return m == ModeDiscord || m == ModeCustom
}

// buildArgs assembles the winws.exe (zapret) command line by wrapping a
// strategy's desync flags with the WinDivert capture filter and the per-group
// --filter-* / --hostlist scoping.
//
// The split of responsibilities is deliberate: the *strategy* (see
// strategies.go) decides *how* the TLS ClientHello / QUIC Initial is mangled so
// a DPI box can't read the SNI; this function decides *what* it applies to and
// wires the two protocol groups together.
//
//   - TCP/443: strat.TCP holds the --dpi-desync* flags (a fake ClientHello, a
//     split, fooling, and repeats — see the Strategy comments).
//   - UDP/443 (QUIC): strat.UDP desyncs the QUIC Initial so the QUIC handshake
//     fails and the browser transparently falls back to the TCP/443 path above,
//     rather than leaking the SNI in a QUIC Initial we aren't fragmenting.
//
// hostlistPath, when non-empty, scopes both groups to the listed domains
// (Discord / Custom sub-modes); pass "" for Full mode to cover all hosts.
func buildArgs(strat Strategy, hostlistPath string) []string {
	args := []string{
		// WinDivert capture filter: only pull the ports we act on off the
		// stack, so we add zero overhead to everything else.
		"--wf-tcp=443",
		"--wf-udp=443",

		// --- TCP/443 desync group ---
		"--filter-tcp=443",
	}
	if hostlistPath != "" {
		args = append(args, "--hostlist="+hostlistPath)
	}
	args = append(args, strat.TCP...)

	// --- UDP/443 (QUIC) desync group ---
	args = append(args, "--new", "--filter-udp=443")
	if hostlistPath != "" {
		args = append(args, "--hostlist="+hostlistPath)
	}
	args = append(args, strat.UDP...)
	return args
}
