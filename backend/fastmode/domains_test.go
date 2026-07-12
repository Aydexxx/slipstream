package fastmode

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestNormalizeDomain(t *testing.T) {
	cases := map[string]string{
		"discord.com":                "discord.com",
		"  Discord.COM ":             "discord.com",
		"https://discord.com/app":    "discord.com",
		"http://www.reddit.com":      "reddit.com",
		"*.discord.com":              "discord.com",
		"user@mail.example.com:443":  "mail.example.com",
		"cdn.discordapp.com/foo?x=1": "cdn.discordapp.com",
		"":                           "",
		"   ":                        "",
		"notadomain":                 "", // no dot
		"has space.com":              "",
		"x.com":                      "x.com",
	}
	for in, want := range cases {
		if got := normalizeDomain(in); got != want {
			t.Errorf("normalizeDomain(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeDomainsDedupsAndOrders(t *testing.T) {
	in := []string{"Discord.com", "https://discord.com/", "reddit.com", "", "bad", "reddit.com"}
	want := []string{"discord.com", "reddit.com"}
	if got := normalizeDomains(in); !reflect.DeepEqual(got, want) {
		t.Errorf("normalizeDomains = %v, want %v", got, want)
	}
}

func TestPresetsAreNonEmptyAndNormalizable(t *testing.T) {
	want := []string{"Discord", "Instagram", "Reddit", "TikTok", "Wikipedia", "X", "YouTube"}
	if got := PresetNames(); !reflect.DeepEqual(got, want) {
		t.Errorf("PresetNames = %v, want %v", got, want)
	}
	for name, domains := range Presets() {
		if len(domains) == 0 {
			t.Errorf("preset %q is empty", name)
		}
		for _, d := range domains {
			if normalizeDomain(d) != d {
				t.Errorf("preset %q domain %q is not already normalized", name, d)
			}
		}
	}
}

func TestPresetsReturnsCopy(t *testing.T) {
	p := Presets()
	p["Discord"][0] = "tampered"
	if presets["Discord"][0] == "tampered" {
		t.Fatal("Presets() leaked a reference to the internal preset slice")
	}
}

func TestCustomListRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom_domains.txt")

	// Missing file → empty, no error.
	got, err := loadCustomDomains(path)
	if err != nil {
		t.Fatalf("loadCustomDomains(missing) error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("loadCustomDomains(missing) = %v, want empty", got)
	}

	if err := saveCustomDomains(path, []string{"HTTPS://Example.com/x", "example.com", "reddit.com", "bad"}); err != nil {
		t.Fatalf("saveCustomDomains error = %v", err)
	}
	got, err = loadCustomDomains(path)
	if err != nil {
		t.Fatalf("loadCustomDomains error = %v", err)
	}
	want := []string{"example.com", "reddit.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round-trip = %v, want %v", got, want)
	}
}
