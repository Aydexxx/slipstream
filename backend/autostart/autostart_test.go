package autostart

import "testing"

// Uses a distinctive, unlikely-to-collide value name under HKCU\...\Run and
// always cleans up after itself, so this is safe to run against the real
// per-user registry.
const testAppName = "SlipstreamAutostartTest-8f21c9d4"

func TestEnableDisableRoundTrip(t *testing.T) {
	t.Cleanup(func() { _ = Disable(testAppName) })

	if enabled, err := IsEnabled(testAppName); err != nil {
		t.Fatalf("IsEnabled before Enable: %v", err)
	} else if enabled {
		t.Fatalf("expected not enabled before Enable")
	}

	if err := Enable(testAppName, `C:\Program Files\Slipstream\slipstream.exe`, "--autostart"); err != nil {
		t.Fatalf("Enable: %v", err)
	}

	enabled, err := IsEnabled(testAppName)
	if err != nil {
		t.Fatalf("IsEnabled after Enable: %v", err)
	}
	if !enabled {
		t.Fatalf("expected enabled after Enable")
	}

	// Re-enabling (e.g. after the exe moved) must overwrite cleanly, not error.
	if err := Enable(testAppName, `C:\Program Files\Slipstream\slipstream.exe`, "--autostart"); err != nil {
		t.Fatalf("re-Enable: %v", err)
	}

	if err := Disable(testAppName); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if enabled, err := IsEnabled(testAppName); err != nil {
		t.Fatalf("IsEnabled after Disable: %v", err)
	} else if enabled {
		t.Fatalf("expected not enabled after Disable")
	}

	// Disable again must be a safe no-op.
	if err := Disable(testAppName); err != nil {
		t.Fatalf("second Disable: %v", err)
	}
}

func TestQuoteArgQuotesOnlyWhenNeeded(t *testing.T) {
	if got := quoteArg("simple"); got != "simple" {
		t.Errorf("quoteArg(simple) = %q", got)
	}
	if got := quoteArg(`C:\Program Files\Slipstream\slipstream.exe`); got != `"C:\Program Files\Slipstream\slipstream.exe"` {
		t.Errorf("quoteArg with space = %q", got)
	}
}
