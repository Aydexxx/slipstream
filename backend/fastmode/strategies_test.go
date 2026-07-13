package fastmode

import "testing"

func TestDefaultStrategyIsMarkedAndUnique(t *testing.T) {
	n := 0
	for _, s := range strategies {
		if s.Default {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("expected exactly one Default strategy, got %d", n)
	}
	if got := defaultStrategy().ID; got == "" {
		t.Error("default strategy has empty ID")
	}
}

func TestStrategyIDsAreUniqueAndComplete(t *testing.T) {
	seen := map[string]struct{}{}
	for _, s := range strategies {
		if s.ID == "" {
			t.Error("strategy has empty ID")
		}
		if _, dup := seen[s.ID]; dup {
			t.Errorf("duplicate strategy ID %q", s.ID)
		}
		seen[s.ID] = struct{}{}
		if len(s.TCP) == 0 || len(s.UDP) == 0 {
			t.Errorf("strategy %q must define both TCP and UDP desync flags", s.ID)
		}
	}
}

func TestResolveStrategyFallsBackToDefault(t *testing.T) {
	if got := resolveStrategy("balanced").ID; got != "balanced" {
		t.Errorf("resolveStrategy(balanced) = %q, want balanced", got)
	}
	// Empty and unknown IDs must both resolve to the default rather than error,
	// so a stale persisted settings value can never wedge Fast Mode.
	if got := resolveStrategy("").ID; got != defaultStrategy().ID {
		t.Errorf("resolveStrategy(\"\") = %q, want default %q", got, defaultStrategy().ID)
	}
	if got := resolveStrategy("no-such-isp").ID; got != defaultStrategy().ID {
		t.Errorf("resolveStrategy(unknown) = %q, want default %q", got, defaultStrategy().ID)
	}
}

func TestBuildArgsUsesStrategyDesyncFlags(t *testing.T) {
	turbo := resolveStrategy("turbo")
	args := buildArgs(turbo, "")
	// Turbo uses a plain split2 with no injected fake on TCP.
	if !contains(args, "--dpi-desync=split2") {
		t.Errorf("expected turbo's TCP desync flag in %v", args)
	}
	// The QUIC group's flags must still be wired after the --new separator.
	if !contains(args, "--filter-udp=443") || !contains(args, "--dpi-desync=fake") {
		t.Errorf("expected the UDP/QUIC desync group in %v", args)
	}
}

func TestStrategiesInfoMirrorsRegistry(t *testing.T) {
	infos := Strategies()
	if len(infos) != len(strategies) {
		t.Fatalf("Strategies() returned %d, want %d", len(infos), len(strategies))
	}
	for i, s := range strategies {
		if infos[i].ID != s.ID || infos[i].Name != s.Name || infos[i].Default != s.Default {
			t.Errorf("Strategies()[%d] = %+v, does not mirror %+v", i, infos[i], s)
		}
	}
}
