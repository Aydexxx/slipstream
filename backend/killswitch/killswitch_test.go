package killswitch

import (
	"net/netip"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarkerPresent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "killswitch.marker")
	if MarkerPresent(path) {
		t.Fatal("marker should be absent initially")
	}

	k := New(path, nil)
	if err := k.writeMarker(Params{EndpointPort: 51820}); err != nil {
		t.Fatalf("writeMarker: %v", err)
	}
	if !MarkerPresent(path) {
		t.Fatal("marker should be present after writeMarker")
	}

	k.deleteMarker()
	if MarkerPresent(path) {
		t.Fatal("marker should be gone after deleteMarker")
	}
}

func TestNewKillSwitchStartsDisarmed(t *testing.T) {
	k := New(filepath.Join(t.TempDir(), "m"), nil)
	if k.IsArmed() {
		t.Error("new kill switch should be disarmed")
	}
}

// Arm must refuse before touching disk or WFP at all when not elevated - the
// "no admin" failure path. Asserted by checking no marker file appears,
// which would only happen if Arm got past the elevation check.
func TestArmRefusesWithoutElevation(t *testing.T) {
	orig := isElevated
	isElevated = func() bool { return false }
	defer func() { isElevated = orig }()

	markerPath := filepath.Join(t.TempDir(), "killswitch.marker")
	k := New(markerPath, nil)

	err := k.Arm(Params{EndpointIP: netip.MustParseAddr("203.0.113.1"), EndpointPort: 51820})
	if err == nil {
		t.Fatal("expected Arm to refuse when not elevated")
	}
	if !strings.Contains(err.Error(), "Administrator") {
		t.Errorf("expected an Administrator-related error, got %v", err)
	}
	if k.IsArmed() {
		t.Error("kill switch must not report armed after a refused Arm")
	}
	if MarkerPresent(markerPath) {
		t.Error("Arm must refuse before writing the marker when not elevated")
	}
}

func TestArmRejectsInvalidEndpointIP(t *testing.T) {
	k := New(filepath.Join(t.TempDir(), "m"), nil)
	if err := k.Arm(Params{EndpointPort: 51820}); err == nil {
		t.Fatal("expected Arm to reject an invalid/zero endpoint IP")
	}
}
