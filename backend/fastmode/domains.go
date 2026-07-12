package fastmode

import (
	"sort"
	"strings"
)

// presets are the bundled domain groups a user can enable without typing
// anything. Each value is a list of registrable/base domains; zapret's
// --hostlist matches a domain and all of its subdomains, so listing the base
// domain (e.g. "discord.com") also covers "gateway.discord.gg"-style hosts
// where they share the base. We deliberately list the extra CDN/media base
// domains a service actually loads from, because a site is only "unblocked"
// once every host its page pulls is also being de-censored.
var presets = map[string][]string{
	"Discord": {
		"discord.com", "discordapp.com", "discordapp.net", "discord.gg",
		"discord.media", "discord.dev", "dis.gd", "discordcdn.com",
	},
	"X": {
		"x.com", "twitter.com", "twimg.com", "t.co",
	},
	"Instagram": {
		"instagram.com", "cdninstagram.com", "fbcdn.net", "instagr.am",
	},
	"Reddit": {
		"reddit.com", "redd.it", "redditstatic.com", "redditmedia.com",
	},
	"TikTok": {
		"tiktok.com", "tiktokcdn.com", "tiktokv.com", "ibytedtos.com",
		"ttwstatic.com", "byteoversea.com",
	},
	"YouTube": {
		"youtube.com", "youtu.be", "ytimg.com", "googlevideo.com",
		"ggpht.com", "youtubei.googleapis.com",
	},
	"Wikipedia": {
		"wikipedia.org", "wikimedia.org", "wiktionary.org", "wikidata.org",
		"wikibooks.org", "wikinews.org",
	},
}

// Presets returns a copy of the bundled preset groups keyed by display name.
// The frontend uses this to render the toggle list; callers must not mutate
// the returned slices.
func Presets() map[string][]string {
	out := make(map[string][]string, len(presets))
	for name, domains := range presets {
		cp := make([]string, len(domains))
		copy(cp, domains)
		out[name] = cp
	}
	return out
}

// PresetNames returns the preset group names in stable alphabetical order.
func PresetNames() []string {
	names := make([]string, 0, len(presets))
	for name := range presets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// discordDomains is the hostlist used by the built-in Discord-only sub-mode.
func discordDomains() []string {
	cp := make([]string, len(presets["Discord"]))
	copy(cp, presets["Discord"])
	return cp
}

// normalizeDomain reduces free-form user input ("HTTPS://Discord.com/app",
// " *.discord.com ") to the bare, lowercased host zapret's hostlist expects
// ("discord.com"). It returns "" for input that has no usable host, so the
// caller can drop it.
func normalizeDomain(raw string) string {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return ""
	}
	// Strip a scheme if the user pasted a URL.
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	// Drop any path, query, port, or userinfo.
	s = strings.TrimPrefix(s, "www.")
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	if i := strings.LastIndex(s, "@"); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.Index(s, ":"); i >= 0 {
		s = s[:i]
	}
	// A leading wildcard ("*.discord.com") is redundant — zapret already
	// matches subdomains of a listed base domain.
	s = strings.TrimPrefix(s, "*.")
	s = strings.Trim(s, ".")
	// Reject anything that clearly isn't a hostname.
	if s == "" || !strings.Contains(s, ".") || strings.ContainsAny(s, " \t") {
		return ""
	}
	return s
}

// normalizeDomains normalizes every entry, drops blanks/invalids, and
// de-duplicates while preserving first-seen order.
func normalizeDomains(raw []string) []string {
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		d := normalizeDomain(r)
		if d == "" {
			continue
		}
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		out = append(out, d)
	}
	return out
}
