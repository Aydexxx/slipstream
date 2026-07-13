package fastmode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func contains(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func countFlag(args []string, prefix string) int {
	n := 0
	for _, a := range args {
		if strings.HasPrefix(a, prefix) {
			n++
		}
	}
	return n
}

func TestBuildArgsFullModeHasNoHostlist(t *testing.T) {
	args := buildArgs(defaultStrategy(), "")
	if !contains(args, "--wf-tcp=443") {
		t.Error("expected --wf-tcp=443 for TLS ClientHello fragmentation")
	}
	if !contains(args, "--wf-udp=443") {
		t.Error("expected --wf-udp=443 for QUIC handling")
	}
	if n := countFlag(args, "--hostlist="); n != 0 {
		t.Errorf("full mode must not scope to a hostlist, got %d hostlist flags", n)
	}
	if !contains(args, "--dpi-desync=fake,split2") {
		t.Error("expected TCP fragmentation desync strategy")
	}
	if !contains(args, "--new") {
		t.Error("expected a --new separator between the TCP and UDP groups")
	}
}

func TestBuildArgsScopedModeHasHostlistInBothGroups(t *testing.T) {
	path := `C:\data\hostlist.txt`
	args := buildArgs(defaultStrategy(), path)
	if n := countFlag(args, "--hostlist="); n != 2 {
		t.Errorf("scoped mode should apply the hostlist to both the TCP and UDP groups, got %d", n)
	}
	if !contains(args, "--hostlist="+path) {
		t.Errorf("expected --hostlist=%s in args %v", path, args)
	}
}

func TestModeHelpers(t *testing.T) {
	if !validMode(ModeFull) || !validMode(ModeDiscord) || !validMode(ModeCustom) {
		t.Error("known modes should validate")
	}
	if validMode(Mode("bogus")) {
		t.Error("unknown mode should not validate")
	}
	if usesHostlist(ModeFull) {
		t.Error("full mode must not use a hostlist")
	}
	if !usesHostlist(ModeDiscord) || !usesHostlist(ModeCustom) {
		t.Error("discord and custom modes must use a hostlist")
	}
}

func TestWriteHostlist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hostlist.txt")
	if err := writeHostlist(path, []string{"Discord.com", "https://discord.com/", "reddit.com"}); err != nil {
		t.Fatalf("writeHostlist error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read hostlist: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "discord.com") || !strings.Contains(got, "reddit.com") {
		t.Errorf("hostlist missing expected domains: %q", got)
	}
	// Deduplicated: "discord.com" should appear exactly once.
	if n := strings.Count(got, "discord.com\r\n"); n != 1 {
		t.Errorf("expected discord.com exactly once, got %d in %q", n, got)
	}
}

func TestWriteHostlistRejectsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hostlist.txt")
	if err := writeHostlist(path, []string{"", "bad", "  "}); err == nil {
		t.Error("expected error when no valid domains are provided")
	}
}
