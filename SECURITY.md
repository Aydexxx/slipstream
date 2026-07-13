# Slipstream Security Model

Slipstream is a single-purpose Windows anti-censorship app: **Fast Mode** —
DPI bypass via `winws.exe`/WinDivert plus encrypted Cloudflare DNS. This
document states exactly what it does to your machine and network, what it
stores, and how to verify that removing it leaves no trace.

## Threat model & guarantees

Slipstream is built to four rules:

1. **Ship nothing untrusted.** Every third-party binary is embedded in the app
   at build time and integrity-verified before use — nothing is downloaded at
   runtime.
2. **Don't leak.** The only outbound connections are the ones you'd expect:
   Cloudflare for DNS, and nothing else. Traffic is never routed through a
   proxy or remote server.
3. **Don't leave persistent changes.** Every system change is reversible, and
   the uninstaller provably returns the machine to its pre-install state.
4. **Don't phone home.** No telemetry, no analytics, no crash reporting, no
   update pings — none.

## Network behavior

**The only outbound connection Slipstream can originate is this one, and
nothing else:**

| # | Destination | When | Origin |
|---|-------------|------|--------|
| 1 | Cloudflare DNS `1.1.1.1` / `1.0.0.1` + DoH template `https://cloudflare-dns.com/dns-query` | While Fast Mode is on | The **Windows resolver**, after Fast Mode points it at Cloudflare (`netsh`). Slipstream itself makes no DNS request here. |

This is verifiable — a repo-wide search for HTTP/DNS primitives (`net/http`,
`http.`, `net.Dial`, `LookupIP`, `https://`) across the Go backend returns only
`fastmode/dns.go` (plus test files). Slipstream originates no HTTP requests of
its own.

- **No auto-update / no runtime downloads.** No update checker, no GitHub
  release fetching, no remote code loading.
- **No telemetry / analytics / crash reporting.** No SDK of any kind in
  `go.mod` or source. Logging is local only.
- **Frontend is 100% local.** It talks to Go exclusively over in-process Wails
  IPC. No `fetch`/`XMLHttpRequest`, no external `<script>`/`<link>`/`<img>`, no
  CDN. Fonts are self-hosted via `@fontsource` (no Google Fonts). Assets are
  served from an embedded filesystem (`go:embed all:frontend/dist`).

## Software integrity

The vendored engine binaries — `winws.exe`, `WinDivert64.sys`, `WinDivert.dll`,
`cygwin1.dll` — are **embedded** into the executable (`assets/assets.go`,
`//go:embed`) and pinned to a hardcoded **SHA-256** manifest
(`backend/engine/manifest.go`).

- On startup, each file is extracted to `%LocalAppData%\Slipstream\engine\` only
  if missing or hash-mismatched; the embedded bytes are re-verified against the
  manifest *before* being written (a corrupt build aborts).
- Before **every** mode launch, `engine.Verify` recomputes the SHA-256 of each
  extracted file and refuses to run the mode on any mismatch.

Trust bottoms out at the hardcoded hashes; their provenance (upstream versions,
URLs, how they were obtained) is documented in `ENGINES.md`. *Accepted residual
(optional future hardening): there is no independent Authenticode signature
check of the third-party exes beyond the SHA-256 pin.*

## Data at rest

Everything Slipstream stores lives under `%LocalAppData%\Slipstream\`. It stores
no secrets — there is no key material anywhere on disk.

| Data | Location | Protection |
|------|----------|------------|
| Last-mode / reconnect prefs | `state\settings.json` | Plaintext — no secrets. |
| Custom domain list, hostlist | `fastmode\*.txt` | Plaintext — no secrets. |
| DNS backup | `fastmode\dns_backup.json` | Plaintext — recovery metadata, no secrets. |
| Logs | `logs\*.log` | Plaintext — no secrets. |

## Elevation

Slipstream runs **fully elevated** (manifest `requireAdministrator` +
self-elevation via `ShellExecute "runas"`). There is no privileged helper /
least-privilege split — it's all-or-nothing, which is honest about the fact that
the core features genuinely require admin:

| Operation | Why admin |
|-----------|-----------|
| WinDivert driver load (Fast Mode) | Loads a kernel driver |
| `netsh` DNS + HKLM DoH policy | System network configuration |

The one deliberately **unelevated** action is autostart: the HKCU `...\Run` key
launches Slipstream unelevated at sign-in, and it then self-elevates (a UAC
prompt each boot). This is inherent to using the Run key rather than a
privileged scheduled task, and is documented in the UI.

## Teardown & crash-safety

Every persistent change has a reversal, and every exit path triggers it. The
state machine's `Shutdown()` is idempotent (`sync.Once`), so overlapping
triggers are safe.

| Exit path | What restores state |
|-----------|---------------------|
| Window close / tray Quit | Wails `OnShutdown` → `sm.Shutdown()` (tears down Fast Mode) |
| Process exit | `defer sm.Shutdown()` safety net in `main.go` |
| Log off / system shutdown | `sessionwatch` catches `WM_QUERYENDSESSION`, runs `sm.Shutdown()` under a shutdown-block reason, time-bounded so it can't hang the OS |
| **Hard kill / power loss** | **Next launch** `sm.Reconcile()` (below) |
| Reset & Quit (in-app / tray) | `sm.Shutdown()` **+** full `cleanup.RestoreNetworkState` |

**Hard-kill resilience.** Fast Mode never blocks traffic, so a hard kill can
never strand you offline — it only leaves the system DNS pointed at Cloudflare.
The next-launch reconciler (`sm.Reconcile()`) unconditionally, in order: kills
any orphaned `winws.exe` and restores DNS from `dns_backup.json`. So a crash
can never leave your resolver permanently overridden past the next start.

Two system changes were historically **not** reversed by any teardown path and
are now fixed (they are reversed on every relevant teardown and by the
uninstaller): the global `netsh dns ... encryption` DoH template and the
WinDivert kernel **service** registration.

## Uninstall — the "zero traces" guarantee

"Reset & Quit" (Settings → Advanced, or the tray) restores networking and
closes the app **without deleting anything**. **Uninstall** (Settings →
Advanced) does everything Reset does and then removes every trace, via a
two-stage self-deleting uninstaller (`slipstream.exe --uninstall`):

1. **Restore network/system state:** DNS, the global DoH template, the WinDivert
   driver service, and orphaned processes.
2. **Remove all persistence:** the HKCU autostart Run key, Start-Menu/Desktop
   shortcuts, any Add/Remove-Programs uninstall entry (matched by display name),
   the entire `%LocalAppData%\Slipstream\` tree (engine binaries, logs,
   settings — everything), and the install directory.
3. **Self-delete:** the running exe copies itself to `%TEMP%`, and the copy
   waits for the app to exit, performs steps 1–2, then deletes itself — so not
   even the uninstaller remains.

To confirm this returns your machine to its exact pre-install baseline, run the
before/after procedure in [`docs/UNINSTALL-VERIFICATION.md`](docs/UNINSTALL-VERIFICATION.md).

## Accepted residual items (optional future hardening)

- **No Authenticode check** of the third-party engine exes beyond the pinned
  SHA-256 (see *Software integrity*).
- **No explicit webview CSP.** The practical attack surface is already nil — the
  frontend bundle has zero external references and no network capability of its
  own — so a Content-Security-Policy would be belt-and-suspenders rather than a
  fix for any concrete gap.

## Reporting

Found a security issue? Please report it privately to the maintainer rather than
opening a public issue.
