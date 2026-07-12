# Slipstream Security Model

Slipstream is a Windows anti-censorship app with two modes: **Fast Mode** (DPI
bypass via `winws.exe`/WinDivert + encrypted Cloudflare DNS) and **Private
Mode** (an obfuscated AmneziaWG tunnel to *your own* VPS, with a WFP kill
switch). This document states exactly what it does to your machine and network,
what it stores, and how to verify that removing it leaves no trace.

## Threat model & guarantees

Slipstream is built to four rules:

1. **Ship nothing untrusted.** Every third-party binary is embedded in the app
   at build time and integrity-verified before use — nothing is downloaded at
   runtime.
2. **Don't leak.** The only outbound connections are the ones you'd expect: your
   own VPS, and Cloudflare for DNS. Private Mode fails **closed** — if the
   tunnel drops, a WFP kill switch blocks all non-tunnel traffic.
3. **Don't leave persistent changes.** Every system change is reversible, and
   the uninstaller provably returns the machine to its pre-install state.
4. **Don't phone home.** No telemetry, no analytics, no crash reporting, no
   update pings — none.

## Network behavior

**The only outbound connections Slipstream can originate are these three, and
nothing else:**

| # | Destination | When | Origin |
|---|-------------|------|--------|
| 1 | Cloudflare DNS `1.1.1.1` / `1.0.0.1` + DoH template `https://cloudflare-dns.com/dns-query` | While Fast Mode is on | The **Windows resolver**, after Fast Mode points it at Cloudflare (`netsh`). Slipstream itself makes no DNS request here. |
| 2 | `https://1.1.1.1/cdn-cgi/trace` | On demand, only while Private Mode is connected | A single gated `GET` to display your tunnel-exit IP. Refuses unless state is Connected, the kill switch is armed, and the handshake is fresh. |
| 3 | Your configured VPS endpoint (`net.LookupIP`) | At Private Mode connect | Resolves your WireGuard server host to an IP once, then pins it so reconnects never need DNS. IP-literal endpoints skip this entirely. |

This is verifiable — a repo-wide search for HTTP/DNS primitives (`net/http`,
`http.`, `net.Dial`, `LookupIP`, `https://`) across the Go backend returns only
`fastmode/dns.go`, `privatemode/externalip.go`, and `privatemode/controller.go`
(plus test files). The only literal `http.Client.Do` in the codebase is the
gated trace call in `externalip.go`.

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
`cygwin1.dll` (Fast Mode); `amneziawg.exe`, `wintun.dll` (Private Mode) — are
**embedded** into the executable (`assets/assets.go`, `//go:embed`) and pinned
to a hardcoded **SHA-256** manifest (`backend/engine/manifest.go`).

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

Everything Slipstream stores lives under `%LocalAppData%\Slipstream\`.

| Data | Location | Protection |
|------|----------|------------|
| AmneziaWG config (contains the tunnel **private key**) | `private\tunnel.conf.dpapi` | **DPAPI**, user scope + app entropy (`Slipstream-PrivateMode-v1`). Unusable on another account or machine. |
| Last-mode / reconnect prefs | `state\settings.json` | Plaintext — no secrets. |
| Custom domain list, hostlist | `fastmode\*.txt` | Plaintext — no secrets. |
| DNS backup, kill-switch marker | `fastmode\dns_backup.json`, `private\killswitch.marker` | Plaintext — recovery metadata, no secrets. |
| Logs | `logs\*.log` | Plaintext — **never contain key material** (see below). |

- **The private key is never logged.** `Config.Raw` is tagged `json:"-"`; log
  calls around config import/parse emit only the endpoint, pinned IP, tunnel
  mode, and domain counts — never the key. Verified by tests in
  `privatemode/store_test.go` and `dpapi/dpapi_test.go`.
- **Transient plaintext:** during a Private Mode connect, the config is briefly
  written as `private\Slipstream.conf` (the AmneziaWG service copies it into its
  own encrypted store) and then **shredded** (overwritten + deleted). A hard
  kill in that window could leave it behind; both startup reconciliation and the
  uninstaller **shred any leftover `Slipstream.conf`** so the key never outlives
  the session that wrote it.

## Elevation

Slipstream runs **fully elevated** (manifest `requireAdministrator` +
self-elevation via `ShellExecute "runas"`). There is no privileged helper /
least-privilege split — it's all-or-nothing, which is honest about the fact that
the core features genuinely require admin:

| Operation | Why admin |
|-----------|-----------|
| WinDivert driver load (Fast Mode) | Loads a kernel driver |
| `netsh` DNS + HKLM DoH policy | System network configuration |
| WFP kill-switch filters | Windows Filtering Platform requires admin |
| AmneziaWG service install | Creates/removes a Windows service |

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
| Window close / tray Quit | Wails `OnShutdown` → `sm.Shutdown()` (tears down active mode) |
| Process exit | `defer sm.Shutdown()` safety net in `main.go` |
| Log off / system shutdown | `sessionwatch` catches `WM_QUERYENDSESSION`, runs `sm.Shutdown()` under a shutdown-block reason, time-bounded so it can't hang the OS |
| **Hard kill / power loss** | **Next launch** `sm.Reconcile()` (below) |
| Reset & Quit (in-app / tray) | `sm.Shutdown()` **+** full `cleanup.RestoreNetworkState` |

**Hard-kill resilience.** The WFP kill switch installs in a *non-dynamic* WFP
session so it fails closed even if the app is force-killed — which means a hard
kill while armed leaves the machine's internet blocked until recovery. The
next-launch reconciler (`sm.Reconcile()`) unconditionally, in order: kills any
orphaned `winws.exe`, restores DNS from `dns_backup.json`, removes all
kill-switch WFP filters (restoring connectivity first), uninstalls any leftover
AmneziaWG tunnel service, and shreds any leftover plaintext config. So a crash
can never strand you offline or leak a key past the next start.

Three system changes were historically **not** reversed by any teardown path and
are now fixed (they are reversed on every relevant teardown and by the
uninstaller): the global `netsh dns ... encryption` DoH template, the WinDivert
kernel **service** registration, and a hard-killed plaintext config.

## Uninstall — the "zero traces" guarantee

"Reset & Quit" (Settings → Advanced, or the tray) restores networking and
closes the app **without deleting anything**. **Uninstall** (Settings →
Advanced) does everything Reset does and then removes every trace, via a
two-stage self-deleting uninstaller (`slipstream.exe --uninstall`):

1. **Restore network/system state:** DNS, the global DoH template, per-interface
   DoH registry keys, all WFP kill-switch filters + provider + sublayer, the
   AmneziaWG tunnel service, the WinDivert driver service, orphaned processes,
   and any leftover plaintext key.
2. **Remove all persistence:** the HKCU autostart Run key, Start-Menu/Desktop
   shortcuts, any Add/Remove-Programs uninstall entry (matched by display name),
   the entire `%LocalAppData%\Slipstream\` tree (engine binaries, configs, logs,
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
