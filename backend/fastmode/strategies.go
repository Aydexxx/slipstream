package fastmode

// This file is the single place to tune Fast Mode's DPI-bypass strategies.
//
// A *strategy* is the "how": the winws.exe (zapret) desync flags that decide
// the way the TLS ClientHello / QUIC Initial is mangled so a DPI box can't read
// the SNI and block the flow. It is orthogonal to the *target* (Mode: Full /
// Discord / Custom), which is the "what" — the hostlist the desync applies to.
// The two combine in buildArgs: the target supplies --hostlist scoping, the
// strategy supplies the --dpi-desync* flags.
//
// Why a set of strategies at all: Turkish DPI is not uniform. Different ISPs
// (and different regions/PoPs within one ISP) run different DPI vendors and
// firmware, so a desync that sails through Türk Telekom may be dropped on
// Superonline and vice-versa. There is no single flag set that works
// everywhere, which is why zapret ships dozens of example configs. Bypass is
// inherently trial-and-error per line, so we ship several named presets and let
// the user switch until one sticks (see the UI's "try another strategy" hint).
//
// Every flag below is a real zapret/winws option; nothing here is invented.
// The presets are combinations of the standard desync primitives:
//
//   --dpi-desync=<methods>     comma-separated desync methods, applied in order:
//                                fake      inject a bogus ClientHello the DPI
//                                          latches onto while the real server
//                                          discards it (bad checksum/seq)
//                                split2    split the real ClientHello into two
//                                          TCP segments so the SNI is never a
//                                          contiguous field
//                                disorder2 split *and* send the segments out of
//                                          order, which also defeats simple
//                                          reassembly-by-arrival DPI
//   --dpi-desync-split-pos=N   where to split: a byte offset (1, 2, …) or a
//                                named marker — we use "midsld", the middle of
//                                the second-level domain, which lands the cut
//                                inside the SNI hostname itself
//   --dpi-desync-fooling=<f>   how the *fake* packet is made to look invalid to
//                                the real server while still fooling the DPI:
//                                badseq (wrong TCP seq) and md5sig (bogus MD5
//                                TCP option) are the two that work reliably
//                                behind WinDivert on Windows; badsum is
//                                deliberately avoided because NIC checksum
//                                offload usually rewrites it before it leaves.
//   --dpi-desync-repeats=N     resend the desync N times so a DPI box that only
//                                samples some packets still sees it.
//
// For UDP/443 (QUIC) we desync the QUIC Initial with a fake so the QUIC
// handshake fails and the browser transparently falls back to the TCP/443 path
// above — rather than leaking the SNI in a QUIC Initial we aren't splitting.
//
// The presets do NOT reference bundled fake-payload files
// (--dpi-desync-fake-tls=<file> / --dpi-desync-fake-quic=<file>): winws
// generates a workable built-in fake on its own, so the strategies stay robust
// regardless of which auxiliary blobs ship with the engine.

// Strategy is one named desync configuration. TCP and UDP hold only the
// --dpi-desync* flags for each protocol group; buildArgs wraps them with the
// WinDivert capture filter, the per-group --filter-*, and any --hostlist.
type Strategy struct {
	ID          string // stable key persisted in settings; never rename
	Name        string // display name
	Group       string // UI grouping ("General" / "Turkish ISPs")
	Description string // one-line hint shown in the picker
	Default     bool   // the preset selected when the user hasn't chosen one
	TCP         []string
	UDP         []string
}

// StrategyInfo is the JSON-serialisable view handed to the frontend. It omits
// the raw flag slices — the UI only needs to name and describe the choice.
type StrategyInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Group       string `json:"group"`
	Description string `json:"description"`
	Default     bool   `json:"default"`
}

// strategies is the ordered, built-in preset list. Order here is the order the
// UI renders them in. Keep the "General" presets first (lightest → heaviest),
// then the ISP-tuned ones.
//
// The ISP presets are starting points based on the desync families each ISP's
// DPI is known to respond to in the zapret community's Turkish configs — not
// guarantees. If the picked one doesn't unblock a site, the user should try the
// next; that trial-and-error is expected and is surfaced in the UI.
var strategies = []Strategy{
	{
		ID:          "turbo",
		Name:        "Turbo",
		Group:       "General",
		Description: "Lightest and fastest — a single ClientHello split, no fake packet. Try first; works on lenient DPI.",
		// Just fragment the SNI; no injected fake, minimal overhead.
		TCP: []string{
			"--dpi-desync=split2",
			"--dpi-desync-split-pos=1",
		},
		UDP: []string{
			"--dpi-desync=fake",
			"--dpi-desync-repeats=2",
		},
	},
	{
		ID:          "balanced",
		Name:        "Balanced",
		Group:       "General",
		Description: "Fake ClientHello plus a split, with badseq fooling. Broadly effective default.",
		Default:     true,
		// The long-standing default: fake+split2 at offset 1, badseq fooling,
		// repeated so sampling DPI still catches it.
		TCP: []string{
			"--dpi-desync=fake,split2",
			"--dpi-desync-split-pos=1",
			"--dpi-desync-fooling=badseq",
			"--dpi-desync-repeats=6",
		},
		UDP: []string{
			"--dpi-desync=fake",
			"--dpi-desync-repeats=6",
		},
	},
	{
		ID:          "aggressive",
		Name:        "Aggressive",
		Group:       "General",
		Description: "Fake plus out-of-order split at the SNI, double fooling, more repeats. For stubborn DPI at some speed cost.",
		// Heaviest general preset: disorder2 (split + reorder) cut inside the
		// SLD, badseq+md5sig fooling, and more repeats.
		TCP: []string{
			"--dpi-desync=fake,disorder2",
			"--dpi-desync-split-pos=midsld",
			"--dpi-desync-fooling=badseq,md5sig",
			"--dpi-desync-repeats=8",
		},
		UDP: []string{
			"--dpi-desync=fake",
			"--dpi-desync-repeats=8",
		},
	},
	{
		ID:          "turk-telekom",
		Name:        "Türk Telekom / TTNet",
		Group:       "Turkish ISPs",
		Description: "Tuned for Türk Telekom's SNI filtering: fake + split with badseq, heavier QUIC desync.",
		// TT's filtering responds well to a fake ClientHello split at the front
		// with badseq; QUIC is desynced harder to force the TCP fallback.
		TCP: []string{
			"--dpi-desync=fake,split2",
			"--dpi-desync-split-pos=1",
			"--dpi-desync-fooling=badseq",
			"--dpi-desync-repeats=6",
		},
		UDP: []string{
			"--dpi-desync=fake",
			"--dpi-desync-repeats=8",
		},
	},
	{
		ID:          "superonline",
		Name:        "Superonline (Turkcell)",
		Group:       "Turkish ISPs",
		Description: "Tuned for Turkcell Superonline: out-of-order split (disorder) with a fake and badseq.",
		// Superonline's DPI is more resistant to plain split; disorder2 (reorder
		// the segments) plus a fake tends to be what gets through.
		TCP: []string{
			"--dpi-desync=fake,disorder2",
			"--dpi-desync-split-pos=1",
			"--dpi-desync-fooling=badseq",
			"--dpi-desync-repeats=6",
		},
		UDP: []string{
			"--dpi-desync=fake",
			"--dpi-desync-repeats=6",
		},
	},
	{
		ID:          "vodafone",
		Name:        "Vodafone",
		Group:       "Turkish ISPs",
		Description: "Tuned for Vodafone: fake + split at the SNI midpoint with badseq and md5sig fooling.",
		// Vodafone runs comparatively aggressive DPI; cut inside the SLD and
		// double up the fooling.
		TCP: []string{
			"--dpi-desync=fake,split2",
			"--dpi-desync-split-pos=midsld",
			"--dpi-desync-fooling=badseq,md5sig",
			"--dpi-desync-repeats=8",
		},
		UDP: []string{
			"--dpi-desync=fake",
			"--dpi-desync-repeats=8",
		},
	},
	{
		ID:          "turknet",
		Name:        "TurkNet",
		Group:       "Turkish ISPs",
		Description: "Tuned for TurkNet's lighter DPI: a fake + split slightly into the record with badseq, fewer repeats.",
		// TurkNet's filtering is comparatively light; a modest fake+split with a
		// small offset and fewer repeats is usually enough and stays fast.
		TCP: []string{
			"--dpi-desync=fake,split2",
			"--dpi-desync-split-pos=2",
			"--dpi-desync-fooling=badseq",
			"--dpi-desync-repeats=4",
		},
		UDP: []string{
			"--dpi-desync=fake",
			"--dpi-desync-repeats=4",
		},
	},
}

// defaultStrategy returns the preset used when none is selected or a persisted
// ID no longer exists. It is the one flagged Default (falling back to the first
// entry, so this never panics even if the flag is ever dropped).
func defaultStrategy() Strategy {
	for _, s := range strategies {
		if s.Default {
			return s
		}
	}
	return strategies[0]
}

// resolveStrategy maps a persisted/selected ID to its Strategy, falling back to
// the default for "" or an unknown ID so an out-of-date settings value can
// never wedge Fast Mode.
func resolveStrategy(id string) Strategy {
	for _, s := range strategies {
		if s.ID == id {
			return s
		}
	}
	return defaultStrategy()
}

// Strategies returns the built-in presets as UI-facing info, in render order.
// The frontend uses this to populate the strategy/ISP picker.
func Strategies() []StrategyInfo {
	out := make([]StrategyInfo, 0, len(strategies))
	for _, s := range strategies {
		out = append(out, StrategyInfo{
			ID:          s.ID,
			Name:        s.Name,
			Group:       s.Group,
			Description: s.Description,
			Default:     s.Default,
		})
	}
	return out
}
