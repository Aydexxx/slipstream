package privatemode

import (
	"os"
	"path/filepath"
	"testing"

	"slipstream/backend/engine"
)

// newTestController builds a real Controller against temp directories. Only
// safe for tests that never reach real tunnel/UAPI/WFP I/O (the not-elevated
// refusal, and Disconnect/Shutdown on a controller that was never connected -
// both of which resolve to a read-only, definitely-absent service check).
func newTestController(t *testing.T) *Controller {
	t.Helper()
	em, err := engine.New(nil)
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	c, err := New(Options{Engine: em, DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("privatemode.New: %v", err)
	}
	return c
}

// Connect must refuse before touching config, the tunnel service, or the
// kill switch at all when not elevated - the "no admin" failure path.
func TestConnectRefusesWithoutElevation(t *testing.T) {
	orig := isElevated
	isElevated = func() bool { return false }
	defer func() { isElevated = orig }()

	c := newTestController(t)
	err := c.Connect()
	if err == nil {
		t.Fatal("expected Connect to refuse when not elevated")
	}
	if !contains(err.Error(), "Administrator") {
		t.Errorf("expected an Administrator-related error, got %v", err)
	}
	if got := c.Status(); got.State != StateDisconnected {
		t.Errorf("state should remain %q after a refused Connect, got %q", StateDisconnected, got.State)
	}
}

// Disconnect on a Controller that was never connected must be a safe, real
// no-op: uninstallTunnel's serviceState() check queries the real Service
// Control Manager for the uniquely-named, definitely-absent Slipstream
// tunnel service - a read-only existence check with nothing to mutate.
func TestDisconnectOnFreshControllerIsSafeNoOp(t *testing.T) {
	c := newTestController(t)
	if err := c.Disconnect(); err != nil {
		t.Fatalf("Disconnect on a fresh controller should be a clean no-op, got %v", err)
	}
	if got := c.Status(); got.State != StateDisconnected {
		t.Errorf("expected state %q, got %q", StateDisconnected, got.State)
	}
}

// Shutdown must be safe to call on a fresh Controller too - the unconditional
// app-exit backstop needs to tolerate "nothing was ever connected".
func TestShutdownOnFreshControllerIsSafeNoOp(t *testing.T) {
	c := newTestController(t)
	c.Shutdown() // must not panic or hang
	if got := c.Status(); got.State != StateDisconnected {
		t.Errorf("expected state %q after Shutdown, got %q", StateDisconnected, got.State)
	}
}

// resolveEndpoint is a real "VPS unreachable" failure path today, no mock
// needed: nonexistent.invalid is an RFC 2606 reserved domain guaranteed
// never to resolve.
func TestResolveEndpointFailsForUnresolvableHost(t *testing.T) {
	_, _, err := resolveEndpoint("nonexistent.invalid:51820")
	if err == nil {
		t.Fatal("expected resolveEndpoint to fail for an unresolvable host")
	}
}

func TestResolveEndpointAcceptsIPLiteral(t *testing.T) {
	ip, port, err := resolveEndpoint("203.0.113.5:51820")
	if err != nil {
		t.Fatalf("resolveEndpoint(IP literal): %v", err)
	}
	if ip.String() != "203.0.113.5" {
		t.Errorf("ip = %v, want 203.0.113.5", ip)
	}
	if port != 51820 {
		t.Errorf("port = %v, want 51820", port)
	}
}

func TestResolveEndpointRejectsBadPort(t *testing.T) {
	if _, _, err := resolveEndpoint("203.0.113.5:not-a-port"); err == nil {
		t.Fatal("expected resolveEndpoint to reject a non-numeric port")
	}
}

func TestResolveEndpointRejectsMissingPort(t *testing.T) {
	if _, _, err := resolveEndpoint("203.0.113.5"); err == nil {
		t.Fatal("expected resolveEndpoint to reject an endpoint with no port")
	}
}

func TestBackoffFor(t *testing.T) {
	if backoffFor(1) != 5e9 {
		t.Errorf("attempt 1 backoff = %v", backoffFor(1))
	}
	if backoffFor(2) != 10e9 || backoffFor(3) != 10e9 {
		t.Errorf("later backoff = %v/%v", backoffFor(2), backoffFor(3))
	}
}

func TestClassifyInstallError(t *testing.T) {
	if got := classifyInstallError(errString("CreateService: Access is denied.")); !contains(got.Error(), "Administrator") {
		t.Errorf("access-denied not classified: %v", got)
	}
	if got := classifyInstallError(errString("service marked for deletion")); !contains(got.Error(), "few seconds") {
		t.Errorf("deletion race not classified: %v", got)
	}
	if got := classifyInstallError(errString("weird")); !contains(got.Error(), "Could not start") {
		t.Errorf("default not classified: %v", got)
	}
}

func TestShredRemovesFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "secret.conf")
	if err := os.WriteFile(p, []byte("PrivateKey = abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	shred(p)
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Errorf("shred did not remove file: %v", err)
	}
}

func TestTurkeyMessageMentionsObfuscationAndEndpoint(t *testing.T) {
	if !contains(turkeyMessage, "obfuscation") || !contains(turkeyMessage, "endpoint") {
		t.Errorf("turkey message not actionable: %q", turkeyMessage)
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
