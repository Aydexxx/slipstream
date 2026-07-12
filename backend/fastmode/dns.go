package fastmode

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// Cloudflare's public resolvers and their DoH template. Switching the system
// resolver to these defeats DNS-based blocking (poisoned A records), and the
// DoH template lets Windows 11 encrypt those queries so the ISP can't see or
// tamper with them on the wire.
const (
	cloudflarePrimary   = "1.1.1.1"
	cloudflareSecondary = "1.0.0.1"
	cloudflareDoHURL    = "https://cloudflare-dns.com/dns-query"
)

// createNoWindow hides the console window netsh/powershell would otherwise
// flash on screen. Value of CREATE_NO_WINDOW.
const createNoWindow = 0x08000000

// dnsInterface captures the DNS state of one internet-facing interface at the
// moment Fast Mode took it over, so it can be put back exactly as it was.
type dnsInterface struct {
	Alias   string   `json:"alias"`
	Index   int      `json:"index"`
	GUID    string   `json:"guid"`
	DHCP    bool     `json:"dhcp"`    // true = DNS was DHCP-assigned, not static
	Servers []string `json:"servers"` // static IPv4 servers, when DHCP is false
}

// dnsBackup is the on-disk record of the pre-Fast-Mode DNS state. Its mere
// existence on disk means "DNS may currently be pointed at Cloudflare and
// still needs restoring" — that is the linchpin of crash-safe teardown.
type dnsBackup struct {
	CreatedAt  time.Time      `json:"created_at"`
	Interfaces []dnsInterface `json:"interfaces"`
}

// commandRunner runs an external command and returns its combined output.
// It is a field on dnsManager so tests can substitute a recorder and assert
// on the exact netsh/powershell invocations without mutating real DNS.
type commandRunner func(ctx context.Context, name string, args ...string) (string, error)

// powershellRunner runs a PowerShell script and returns stdout.
type powershellRunner func(ctx context.Context, script string) (string, error)

// dnsManager applies and restores the Cloudflare DoH takeover, backing the
// original state to disk so any teardown path (Stop, app exit, crash, or a
// later cold start) can restore it.
type dnsManager struct {
	backupPath string
	log        *slog.Logger
	run        commandRunner
	powershell powershellRunner
}

func newDNSManager(backupPath string, log *slog.Logger) *dnsManager {
	return &dnsManager{
		backupPath: backupPath,
		log:        log,
		run:        runCommand,
		powershell: runPowerShell,
	}
}

// runCommand is the production commandRunner: it executes name with args,
// hiding the console window, and returns combined stdout+stderr.
func runCommand(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// runPowerShell is the production powershellRunner.
func runPowerShell(ctx context.Context, script string) (string, error) {
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// captureScript enumerates every internet-facing interface (one with an IPv4
// default gateway) and reports its current IPv4 DNS servers plus whether that
// DNS was DHCP-assigned or statically configured. DHCP vs static is read from
// the authoritative registry NameServer value rather than parsed out of
// localized netsh text, so it works on any Windows display language.
const captureScript = `
$ErrorActionPreference = 'SilentlyContinue'
$result = Get-NetIPConfiguration | Where-Object { $_.IPv4DefaultGateway -ne $null } | ForEach-Object {
  $cfg = $_
  $guid = $cfg.NetAdapter.InterfaceGuid
  $ns = (Get-ItemProperty "HKLM:\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces\$guid" -Name NameServer).NameServer
  $servers = @($cfg.DNSServer | Where-Object { $_.AddressFamily -eq 2 } | ForEach-Object { $_.ServerAddresses })
  [PSCustomObject]@{
    alias   = $cfg.InterfaceAlias
    index   = [int]$cfg.InterfaceIndex
    guid    = $guid
    dhcp    = [string]::IsNullOrEmpty($ns)
    servers = $servers
  }
}
ConvertTo-Json -InputObject @($result) -Depth 4
`

// capture reads the current DNS state of all internet-facing interfaces.
func (d *dnsManager) capture(ctx context.Context) ([]dnsInterface, error) {
	out, err := d.powershell(ctx, captureScript)
	if err != nil {
		return nil, fmt.Errorf("read current DNS configuration: %w (%s)", err, strings.TrimSpace(out))
	}
	return parseCaptured(out)
}

// parseCaptured decodes the JSON emitted by captureScript. Windows PowerShell
// 5.1 collapses a single-element array to a bare object, so we accept either
// shape.
func parseCaptured(out string) ([]dnsInterface, error) {
	s := strings.TrimSpace(out)
	if s == "" {
		return nil, nil
	}
	var many []dnsInterface
	if err := json.Unmarshal([]byte(s), &many); err == nil {
		return many, nil
	}
	var one dnsInterface
	if err := json.Unmarshal([]byte(s), &one); err != nil {
		return nil, fmt.Errorf("parse DNS configuration JSON: %w", err)
	}
	return []dnsInterface{one}, nil
}

// apply captures the current DNS state, writes it to the backup file, and
// then switches every internet-facing interface to Cloudflare with DoH. The
// backup is written and flushed to disk *before* the first mutation, so even
// a crash between the two leaves a restorable record.
func (d *dnsManager) apply(ctx context.Context) error {
	ifaces, err := d.capture(ctx)
	if err != nil {
		return err
	}
	if len(ifaces) == 0 {
		return fmt.Errorf("no internet-facing network interface found to apply encrypted DNS")
	}

	if err := d.writeBackup(ifaces); err != nil {
		return err
	}

	// Register Cloudflare as a known DoH server system-wide (idempotent).
	d.addDoHEncryption(ctx, cloudflarePrimary)
	d.addDoHEncryption(ctx, cloudflareSecondary)

	var failed []string
	for _, ifc := range ifaces {
		if err := d.setCloudflare(ctx, ifc); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", ifc.Alias, err))
			continue
		}
		// Best-effort: force Windows to use DoH for Cloudflare on this
		// interface. If it fails the DNS is still Cloudflare (the main win),
		// just not encrypted, so we log and continue rather than abort.
		if err := enableDoHForInterface(ifc.GUID); err != nil && d.log != nil {
			d.log.Warn("could not enable DoH on interface", "alias", ifc.Alias, "error", err)
		}
	}
	d.flushDNS(ctx)

	// If we couldn't switch a single interface, undo whatever we did manage
	// and surface the failure — a half-applied state is worse than none.
	if len(failed) == len(ifaces) {
		_ = d.restore(ctx)
		return fmt.Errorf("failed to apply encrypted DNS to any interface: %s", strings.Join(failed, "; "))
	}
	if len(failed) > 0 && d.log != nil {
		d.log.Warn("encrypted DNS applied with partial failures", "failures", strings.Join(failed, "; "))
	}
	return nil
}

// setCloudflare points one interface's IPv4 DNS at Cloudflare.
func (d *dnsManager) setCloudflare(ctx context.Context, ifc dnsInterface) error {
	for _, inv := range setCloudflareCommands(ifc.Alias) {
		if out, err := d.run(ctx, inv[0], inv[1:]...); err != nil {
			return fmt.Errorf("%s: %w (%s)", strings.Join(inv, " "), err, strings.TrimSpace(out))
		}
	}
	return nil
}

// setCloudflareCommands is the pure list of netsh invocations that switch an
// interface to Cloudflare. Split out so it can be unit-tested without exec.
func setCloudflareCommands(alias string) [][]string {
	name := "name=" + alias
	return [][]string{
		{"netsh", "interface", "ipv4", "set", "dnsservers", name, "static", cloudflarePrimary, "primary", "validate=no"},
		{"netsh", "interface", "ipv4", "add", "dnsservers", name, cloudflareSecondary, "index=2", "validate=no"},
	}
}

// addDoHEncryption registers a Cloudflare IP as a DoH-capable server. Errors
// are non-fatal (the server may already be registered, which is fine).
func (d *dnsManager) addDoHEncryption(ctx context.Context, server string) {
	inv := addDoHEncryptionCommand(server)
	if out, err := d.run(ctx, inv[0], inv[1:]...); err != nil && d.log != nil {
		d.log.Debug("netsh dns add encryption returned error (often benign)", "server", server, "output", strings.TrimSpace(out))
	}
}

func addDoHEncryptionCommand(server string) []string {
	return []string{"netsh", "dns", "add", "encryption",
		"server=" + server, "dohtemplate=" + cloudflareDoHURL,
		"autoupgrade=yes", "udpfallback=no"}
}

// removeDoHEncryptionCommand is the exact inverse of addDoHEncryptionCommand:
// it removes the system-wide Cloudflare DoH server template that apply()
// registered, so uninstalling Fast Mode leaves no global DNS-encryption trace.
func removeDoHEncryptionCommand(server string) []string {
	return []string{"netsh", "dns", "delete", "encryption", "server=" + server}
}

// removeDoHTemplates removes the system-wide DoH server templates for both
// Cloudflare IPs. Best-effort: an absent template returns an error from netsh
// which we log and ignore, since "already gone" is success for teardown.
func (d *dnsManager) removeDoHTemplates(ctx context.Context) {
	for _, server := range []string{cloudflarePrimary, cloudflareSecondary} {
		inv := removeDoHEncryptionCommand(server)
		if out, err := d.run(ctx, inv[0], inv[1:]...); err != nil && d.log != nil {
			d.log.Debug("netsh dns delete encryption returned error (often benign)", "server", server, "output", strings.TrimSpace(out))
		}
	}
}

// RemoveGlobalDoHTemplate removes the system-wide Cloudflare DoH server
// templates Fast Mode registers, independent of any DNS backup. Exposed for
// the cleanup/uninstall path, which must reverse this trace even when no
// dns_backup.json exists (a normal restore already removes it; this is the
// belt-and-suspenders entry point).
func RemoveGlobalDoHTemplate(log *slog.Logger) {
	dm := newDNSManager("", log)
	dm.removeDoHTemplates(context.Background())
}

// restore puts DNS back exactly as the backup recorded it, then deletes the
// backup. It is bulletproof by construction: it attempts *every* recorded
// interface even if some fail, and only removes the backup (the "restore
// still pending" marker) once the pass completes with no hard errors — so a
// crash or partial failure leaves the marker in place for the next start-up
// or teardown path to finish the job. A missing backup is a no-op success.
func (d *dnsManager) restore(ctx context.Context) error {
	backup, err := d.readBackup()
	if err != nil {
		return err
	}
	if backup == nil {
		return nil // nothing was applied / already restored
	}

	var failed []string
	for _, ifc := range backup.Interfaces {
		if err := d.restoreInterface(ctx, ifc); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", ifc.Alias, err))
		}
		// Remove the DoH override we may have added; harmless if absent.
		_ = removeDoHForInterface(ifc.GUID)
	}
	// Remove the system-wide DoH server templates apply() registered, so a
	// restored machine carries no global Cloudflare DNS-encryption trace.
	d.removeDoHTemplates(ctx)
	d.flushDNS(ctx)

	if len(failed) > 0 {
		// Keep the backup so a later attempt can retry — never lose the
		// ability to get the user off Cloudflare.
		return fmt.Errorf("restored DNS with errors (backup kept for retry): %s", strings.Join(failed, "; "))
	}

	if err := os.Remove(d.backupPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove DNS backup after restore: %w", err)
	}
	return nil
}

// restoreInterface reverts a single interface to its recorded state.
func (d *dnsManager) restoreInterface(ctx context.Context, ifc dnsInterface) error {
	for _, inv := range restoreCommands(ifc) {
		if out, err := d.run(ctx, inv[0], inv[1:]...); err != nil {
			return fmt.Errorf("%s: %w (%s)", strings.Join(inv, " "), err, strings.TrimSpace(out))
		}
	}
	return nil
}

// restoreCommands is the pure list of netsh invocations that revert one
// interface. DHCP-assigned DNS is reset to DHCP; static DNS is written back
// server-by-server. A recorded-static-but-empty list can't be reconstructed,
// so it falls back to DHCP (the safe default that regains connectivity).
func restoreCommands(ifc dnsInterface) [][]string {
	name := "name=" + ifc.Alias
	if ifc.DHCP || len(ifc.Servers) == 0 {
		return [][]string{
			{"netsh", "interface", "ipv4", "set", "dnsservers", name, "dhcp", "validate=no"},
		}
	}
	cmds := [][]string{
		{"netsh", "interface", "ipv4", "set", "dnsservers", name, "static", ifc.Servers[0], "primary", "validate=no"},
	}
	for i, srv := range ifc.Servers[1:] {
		cmds = append(cmds, []string{"netsh", "interface", "ipv4", "add", "dnsservers", name, srv, fmt.Sprintf("index=%d", i+2), "validate=no"})
	}
	return cmds
}

func (d *dnsManager) flushDNS(ctx context.Context) {
	if _, err := d.run(ctx, "ipconfig", "/flushdns"); err != nil && d.log != nil {
		d.log.Debug("ipconfig /flushdns failed", "error", err)
	}
}

// pending reports whether a DNS backup exists on disk (i.e. a restore is
// owed). Used at start-up to detect a previous run that never cleaned up.
func (d *dnsManager) pending() bool {
	_, err := os.Stat(d.backupPath)
	return err == nil
}

func (d *dnsManager) writeBackup(ifaces []dnsInterface) error {
	b := dnsBackup{CreatedAt: time.Now(), Interfaces: ifaces}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("encode DNS backup: %w", err)
	}
	tmp := d.backupPath + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create DNS backup: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return fmt.Errorf("write DNS backup: %w", err)
	}
	// Flush to stable storage before the rename so a crash can't leave a
	// truncated backup — this file is the one thing that must survive.
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("sync DNS backup: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close DNS backup: %w", err)
	}
	if err := os.Rename(tmp, d.backupPath); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("commit DNS backup: %w", err)
	}
	return nil
}

// readBackup loads the backup, returning (nil, nil) when none exists.
func (d *dnsManager) readBackup() (*dnsBackup, error) {
	data, err := os.ReadFile(d.backupPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read DNS backup: %w", err)
	}
	var b dnsBackup
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("decode DNS backup %s: %w", d.backupPath, err)
	}
	return &b, nil
}
