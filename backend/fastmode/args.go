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

// buildArgs assembles the winws.exe (zapret) command line.
//
// The strategy fragments/fakes the TLS ClientHello so a DPI box can't read
// the SNI and decide to block the flow:
//
//   - TCP/443: send a bogus "fake" ClientHello with a bad TCP sequence number
//     (badseq) — the DPI inspects it and latches onto the fake SNI, while the
//     real server discards it — then split the genuine ClientHello (split2 at
//     offset 1) so the SNI never appears as a contiguous field. repeats=6
//     hardens this against DPI boxes that only sample some packets.
//   - UDP/443 (QUIC): desync the QUIC Initial so the QUIC handshake fails and
//     the browser transparently falls back to the TCP/443 path above, rather
//     than leaking the SNI in a QUIC Initial we aren't fragmenting.
//
// hostlistPath, when non-empty, scopes both groups to the listed domains
// (Discord / Custom sub-modes); pass "" for Full mode to cover all hosts.
//
// This default is tuned to be broadly effective but DPI implementations vary
// by ISP; the flag set is intentionally centralised here so it can be tuned
// in one place without touching the controller.
func buildArgs(hostlistPath string) []string {
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
	args = append(args,
		"--dpi-desync=fake,split2",
		"--dpi-desync-split-pos=1",
		"--dpi-desync-fooling=badseq",
		"--dpi-desync-repeats=6",

		// --- UDP/443 (QUIC) desync group ---
		"--new",
		"--filter-udp=443",
	)
	if hostlistPath != "" {
		args = append(args, "--hostlist="+hostlistPath)
	}
	args = append(args,
		"--dpi-desync=fake",
		"--dpi-desync-repeats=6",
	)
	return args
}
