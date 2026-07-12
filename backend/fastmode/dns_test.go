package fastmode

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// recordingRunner captures every command a dnsManager would execute, so we can
// assert on the exact netsh/ipconfig calls without mutating the real machine.
type recordingRunner struct {
	mu    sync.Mutex
	calls [][]string
	fail  map[string]bool // join(" ") of an invocation that should return an error
}

func (r *recordingRunner) run(_ context.Context, name string, args ...string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	inv := append([]string{name}, args...)
	r.calls = append(r.calls, inv)
	if r.fail != nil && r.fail[strings.Join(inv, " ")] {
		return "boom", os.ErrPermission
	}
	return "", nil
}

func (r *recordingRunner) sawPrefix(prefix string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.calls {
		if strings.HasPrefix(strings.Join(c, " "), prefix) {
			return true
		}
	}
	return false
}

func newTestDNS(t *testing.T, rec *recordingRunner) *dnsManager {
	t.Helper()
	return &dnsManager{
		backupPath: filepath.Join(t.TempDir(), "dns_backup.json"),
		run:        rec.run,
		powershell: func(context.Context, string) (string, error) { return "", nil },
	}
}

func TestSetCloudflareCommands(t *testing.T) {
	cmds := setCloudflareCommands("Wi-Fi")
	if len(cmds) != 2 {
		t.Fatalf("want 2 netsh commands, got %d", len(cmds))
	}
	if !contains(cmds[0], "static") || !contains(cmds[0], cloudflarePrimary) || !contains(cmds[0], "name=Wi-Fi") {
		t.Errorf("primary set command wrong: %v", cmds[0])
	}
	if !contains(cmds[1], cloudflareSecondary) || !contains(cmds[1], "index=2") {
		t.Errorf("secondary add command wrong: %v", cmds[1])
	}
}

func TestRemoveDoHEncryptionCommandInvertsAdd(t *testing.T) {
	add := addDoHEncryptionCommand(cloudflarePrimary)
	del := removeDoHEncryptionCommand(cloudflarePrimary)
	if !contains(add, "add") || !contains(add, "server="+cloudflarePrimary) {
		t.Fatalf("add command wrong: %v", add)
	}
	if !contains(del, "delete") || !contains(del, "encryption") || !contains(del, "server="+cloudflarePrimary) {
		t.Errorf("delete command wrong: %v", del)
	}
	// The delete form must NOT carry a dohtemplate (netsh delete takes only the server).
	for _, a := range del {
		if strings.HasPrefix(a, "dohtemplate=") {
			t.Errorf("delete command should not include a template: %v", del)
		}
	}
}

// Restore must also remove the system-wide DoH templates apply() registered,
// so no global DNS-encryption trace survives teardown.
func TestRestoreRemovesGlobalDoHTemplates(t *testing.T) {
	rec := &recordingRunner{}
	dm := newTestDNS(t, rec)
	dm.writeBackup([]dnsInterface{{Alias: "Wi-Fi", DHCP: true}})

	if err := dm.restore(context.Background()); err != nil {
		t.Fatalf("restore error = %v", err)
	}
	if !rec.sawPrefix("netsh dns delete encryption server=" + cloudflarePrimary) {
		t.Errorf("primary DoH template was not removed on restore")
	}
	if !rec.sawPrefix("netsh dns delete encryption server=" + cloudflareSecondary) {
		t.Errorf("secondary DoH template was not removed on restore")
	}
}

func TestRestoreCommandsDHCP(t *testing.T) {
	cmds := restoreCommands(dnsInterface{Alias: "Wi-Fi", DHCP: true})
	if len(cmds) != 1 || !contains(cmds[0], "dhcp") {
		t.Errorf("DHCP restore should be a single dhcp reset, got %v", cmds)
	}
}

func TestRestoreCommandsStatic(t *testing.T) {
	cmds := restoreCommands(dnsInterface{Alias: "Ethernet", Servers: []string{"9.9.9.9", "8.8.8.8"}})
	if len(cmds) != 2 {
		t.Fatalf("want set primary + add secondary, got %d", len(cmds))
	}
	if !contains(cmds[0], "static") || !contains(cmds[0], "9.9.9.9") {
		t.Errorf("primary restore wrong: %v", cmds[0])
	}
	if !contains(cmds[1], "8.8.8.8") || !contains(cmds[1], "index=2") {
		t.Errorf("secondary restore wrong: %v", cmds[1])
	}
}

func TestRestoreCommandsStaticButEmptyFallsBackToDHCP(t *testing.T) {
	cmds := restoreCommands(dnsInterface{Alias: "Ethernet", DHCP: false, Servers: nil})
	if len(cmds) != 1 || !contains(cmds[0], "dhcp") {
		t.Errorf("empty static list should fall back to dhcp, got %v", cmds)
	}
}

func TestParseCapturedArrayAndSingle(t *testing.T) {
	arr := `[{"alias":"Wi-Fi","index":5,"guid":"{abc}","dhcp":true,"servers":[]}]`
	got, err := parseCaptured(arr)
	if err != nil || len(got) != 1 || got[0].Alias != "Wi-Fi" || !got[0].DHCP {
		t.Fatalf("parseCaptured(array) = %v, err=%v", got, err)
	}

	// PowerShell 5.1 collapses a one-element array to a bare object.
	single := `{"alias":"Ethernet","index":9,"guid":"{d}","dhcp":false,"servers":["1.2.3.4"]}`
	got, err = parseCaptured(single)
	if err != nil || len(got) != 1 || got[0].Alias != "Ethernet" || len(got[0].Servers) != 1 {
		t.Fatalf("parseCaptured(single) = %v, err=%v", got, err)
	}

	if got, err := parseCaptured("   "); err != nil || got != nil {
		t.Fatalf("parseCaptured(empty) = %v, err=%v", got, err)
	}
}

func TestBackupRoundTripAndPending(t *testing.T) {
	rec := &recordingRunner{}
	dm := newTestDNS(t, rec)

	if dm.pending() {
		t.Fatal("no backup should exist yet")
	}
	ifaces := []dnsInterface{
		{Alias: "Wi-Fi", Index: 5, GUID: "{abc}", DHCP: true},
		{Alias: "Ethernet", Index: 9, GUID: "{def}", Servers: []string{"9.9.9.9"}},
	}
	if err := dm.writeBackup(ifaces); err != nil {
		t.Fatalf("writeBackup error = %v", err)
	}
	if !dm.pending() {
		t.Fatal("pending() should be true after writeBackup")
	}
	b, err := dm.readBackup()
	if err != nil || b == nil {
		t.Fatalf("readBackup error = %v", err)
	}
	if !reflect.DeepEqual(b.Interfaces, ifaces) {
		t.Errorf("backup round-trip mismatch:\n got %v\nwant %v", b.Interfaces, ifaces)
	}
}

// The core guarantee: a clean restore reverts every interface and then deletes
// the backup marker so it is not endlessly re-applied.
func TestRestoreRevertsAllInterfacesAndClearsBackup(t *testing.T) {
	rec := &recordingRunner{}
	dm := newTestDNS(t, rec)
	dm.writeBackup([]dnsInterface{
		{Alias: "Wi-Fi", DHCP: true},
		{Alias: "Ethernet", Servers: []string{"9.9.9.9"}},
	})

	if err := dm.restore(context.Background()); err != nil {
		t.Fatalf("restore error = %v", err)
	}
	if !rec.sawPrefix("netsh interface ipv4 set dnsservers name=Wi-Fi dhcp") {
		t.Error("Wi-Fi was not reset to DHCP")
	}
	if !rec.sawPrefix("netsh interface ipv4 set dnsservers name=Ethernet static 9.9.9.9") {
		t.Error("Ethernet static DNS was not restored")
	}
	if !rec.sawPrefix("ipconfig /flushdns") {
		t.Error("DNS cache was not flushed after restore")
	}
	if dm.pending() {
		t.Error("backup marker should be deleted after a clean restore")
	}
}

// If a restore command fails, the backup must be KEPT so a later teardown or
// the next start-up can retry — never lose the ability to un-hijack DNS.
func TestRestoreKeepsBackupOnFailure(t *testing.T) {
	rec := &recordingRunner{fail: map[string]bool{
		"netsh interface ipv4 set dnsservers name=Wi-Fi dhcp validate=no": true,
	}}
	dm := newTestDNS(t, rec)
	dm.writeBackup([]dnsInterface{{Alias: "Wi-Fi", DHCP: true}})

	if err := dm.restore(context.Background()); err == nil {
		t.Fatal("expected restore to report an error")
	}
	if !dm.pending() {
		t.Error("backup must be retained after a failed restore so it can be retried")
	}
}

func TestRestoreNoBackupIsNoOp(t *testing.T) {
	rec := &recordingRunner{}
	dm := newTestDNS(t, rec)
	if err := dm.restore(context.Background()); err != nil {
		t.Fatalf("restore with no backup should be a no-op, got %v", err)
	}
	if len(rec.calls) != 0 {
		t.Errorf("no commands should run when there is no backup, got %v", rec.calls)
	}
}
