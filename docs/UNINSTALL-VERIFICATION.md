# Uninstall verification

This procedure proves that uninstalling Slipstream returns the machine to its
pre-install network + filesystem state, leaving zero traces. It captures a
snapshot of every surface Slipstream touches, before install and after
uninstall, and diffs them.

Run all commands in an **elevated PowerShell** (Slipstream's changes are
system-scoped). The snapshot is read-only and safe to run at any time.

## 1. Snapshot script

Save as `slipstream-snapshot.ps1`:

```powershell
param([Parameter(Mandatory=$true)][string]$Label)  # e.g. "before" or "after"

$out = "$env:TEMP\slipstream-verify-$Label.txt"
"=== Slipstream state snapshot: $Label @ $(Get-Date -Format o) ===" | Out-File $out

function Section($title, $script) {
    "`n--- $title ---" | Out-File $out -Append
    try { & $script 2>&1 | Out-File $out -Append } catch { "ERROR: $_" | Out-File $out -Append }
}

# Filesystem
Section "AppData dir exists" { Test-Path "$env:LOCALAPPDATA\Slipstream" }
Section "AppData tree" { if (Test-Path "$env:LOCALAPPDATA\Slipstream") { Get-ChildItem -Recurse "$env:LOCALAPPDATA\Slipstream" | Select-Object FullName } }

# Autostart Run key
Section "HKCU Run\Slipstream" { (Get-ItemProperty "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run" -Name Slipstream -ErrorAction SilentlyContinue).Slipstream }

# Add/Remove Programs entry
Section "Uninstall entries named Slipstream" {
    @("HKCU:","HKLM:") | ForEach-Object {
        Get-ChildItem "$_\Software\Microsoft\Windows\CurrentVersion\Uninstall" -ErrorAction SilentlyContinue |
            ForEach-Object { $p = Get-ItemProperty $_.PSPath; if ($p.DisplayName -eq "Slipstream") { $_.PSPath } }
    }
}

# Services
Section "WinDivert service" { sc.exe query WinDivert }
Section "AmneziaWG tunnel service" { sc.exe query 'AmneziaWGTunnel$Slipstream' }

# DNS + DoH
Section "IPv4 DNS servers" { netsh interface ipv4 show dnsservers }
Section "DoH encryption templates" { netsh dns show encryption }

# WFP kill-switch objects (provider/sublayer named "Slipstream Kill Switch")
Section "WFP Slipstream providers" { netsh wfp show state file=- 2>$null | Select-String "Slipstream" }

# Shortcuts
Section "Shortcuts" {
    @("$env:APPDATA\Microsoft\Windows\Start Menu\Programs\Slipstream.lnk",
      "$env:ProgramData\Microsoft\Windows\Start Menu\Programs\Slipstream.lnk",
      "$env:USERPROFILE\Desktop\Slipstream.lnk",
      "$env:PUBLIC\Desktop\Slipstream.lnk") | ForEach-Object { "$_ : $(Test-Path $_)" }
}

Write-Host "Snapshot written to $out"
```

## 2. Procedure

1. **Baseline (before installing):**
   ```powershell
   .\slipstream-snapshot.ps1 -Label before
   ```
2. Install and use Slipstream: run **Fast Mode** at least once (exercises DNS +
   DoH + WinDivert), and connect **Private Mode** at least once (exercises the
   tunnel service + WFP kill switch). Enable **Start with Windows** so the Run
   key is present. Then turn everything off.
3. **Uninstall** via Settings → Advanced → *Uninstall*, and confirm.
4. Wait ~10 seconds for the self-deleting finalizer to complete (watch
   `%TEMP%\slipstream-uninstall.log` for `uninstall completed cleanly`). Then:
   ```powershell
   .\slipstream-snapshot.ps1 -Label after
   ```
5. **Diff:**
   ```powershell
   Compare-Object (Get-Content "$env:TEMP\slipstream-verify-before.txt") `
                  (Get-Content "$env:TEMP\slipstream-verify-after.txt")
   ```

## 3. Pass criteria

The `after` snapshot must match `before`:

- `AppData dir exists` → **False**; the AppData tree is gone.
- `HKCU Run\Slipstream` → **empty**.
- `Uninstall entries named Slipstream` → **none**.
- `WinDivert service` → `1060: service does not exist` (or matches the
  pre-install baseline if you had WinDivert from another tool).
- `AmneziaWG tunnel service` → `1060: service does not exist`.
- `IPv4 DNS servers` → back to your original resolver (DHCP or your static
  servers), **not** `1.1.1.1`/`1.0.0.1`.
- `DoH encryption templates` → no Cloudflare template that wasn't there before.
- `WFP Slipstream providers` → **no matches**.
- `Shortcuts` → all **False**.
- `slipstream.exe` and its install directory are gone; the uninstaller helper in
  `%TEMP%` has deleted itself.

Any surviving line that differs from the `before` snapshot is a leftover trace
and a bug.

## Notes

- The kill switch is intentionally **fail-closed** (non-dynamic WFP session). If
  you force-kill the app while Private Mode is armed, connectivity stays blocked
  until you relaunch Slipstream (its startup reconciler removes the filters) or
  reboot — this is by design, not a leftover.
- Automated unit tests cover the uninstaller's pure logic (command construction,
  path/flag handling, best-effort orchestration). The live system teardown above
  is a manual step because it mutates real WFP/DNS/service/registry state, which
  is unsafe to exercise in CI.
