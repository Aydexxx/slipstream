# Changelog

All notable changes to Slipstream are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

Release validation and hardening pass ahead of 1.0.0.

### Added

- Go test coverage for previously-untested failure paths: not-elevated
  refusal for Fast Mode/Private Mode/kill-switch (`Start`/`Connect`/`Arm`),
  VPS-unreachable DNS resolution failure, the crash-recovery entry point
  (`RecoverPendingDNS`), `classifyLaunchError`'s WinDivert failure
  classification, and `MaybeReconnectLastMode`'s resume-on-launch behavior.
  Coverage: `fastmode` 27.0%→37.5%, `privatemode` 26.6%→33.5%, `killswitch`
  18.3%→19.3%, `statemachine` 67.4%→74.0%.
- Real-`Controller` integration tests (not just fakes) for the idempotent
  no-op teardown guarantee: `fastmode.Controller.Stop`/`Shutdown` and
  `privatemode.Controller.Disconnect`/`Shutdown` on a controller that was
  never started/connected.
- `docs/E2E-TESTING.md` — the six release-gate scenarios (real blocked site,
  Private Mode IP change, forced kill-switch drop, rapid mode switching,
  crash recovery, reboot with auto-reconnect) with pass/fail criteria.
- `docs/PERFORMANCE.md` + `tools/perfcheck.ps1` — ping/throughput measurement
  methodology for confirming Fast Mode's negligible overhead and quantifying
  Private Mode's.
- `tools/fastmode-smoketest` — a standalone harness driving the real
  `backend/fastmode` package to smoke-test the DNS hijack/restore mechanism
  without needing a censored network.

### Fixed

- `FastModePanel`: starting Custom Mode could silently fail with no user
  feedback if fetching the saved domain list failed (an unhandled promise
  rejection outside the app's normal error-reporting path).
- `ImportConfigForm`/`PresetButtons`: replaced a blank gap during loading
  with a visible spinner.
- `PrivateModePanel`: could briefly show an actionable "Connect" button
  before the first status snapshot and config-loaded check had both
  resolved.

### Changed

- Error message copy standardized across `statemachine`, `fastmode`,
  `privatemode`, `killswitch`, and `autostart` — consistent capitalization
  ("Fast Mode"/"Private Mode" as proper names), and plain-English leads on
  the Windows-startup-entry errors that surface verbatim in Settings.

## [0.9.0] - 2026-07-12

First packaged pre-release. Feature-complete; pending end-to-end testing and
polish before 1.0.0.

### Added

- **Fast Mode** — DPI-evasion via `winws.exe`/WinDivert with encrypted
  Cloudflare DoH DNS, crash-safe DNS restore, and integrity-verified engine
  binaries.
- **Private Mode** — an obfuscated AmneziaWG tunnel to the user's own VPS,
  with a fail-closed WFP kill switch and automatic reconnect handling.
- **Unified state machine** — a single `StateManager` coordinating Fast Mode,
  Private Mode, and the kill switch, with mutual exclusion, idempotent
  teardown on every exit path, and startup reconciliation after a crash.
- **Desktop UI** — a Fast/Private/Off mode selector, live connection status,
  settings, and light/dark theming.
- **System tray** — background operation with tray-driven mode toggles,
  close-to-tray, autostart, and Windows session-end handling so a logoff or
  shutdown never leaves network state dirty.
- **Security hardening & trace-free uninstall** — an audited network/data
  surface (documented in `SECURITY.md`), DPAPI-encrypted secrets at rest, and
  a "Reset & Quit" / full uninstall path that restores DNS/routes/firewall
  state and removes every persistent trace (verification steps in
  `docs/UNINSTALL-VERIFICATION.md`).
- **Packaging** — this release: a reproducible build script (`build.ps1`)
  producing a portable `slipstream.exe` and an Inno Setup installer
  (`SlipstreamSetup.exe`), with semantic version stamping and an optional
  code-signing hook.

[Unreleased]: https://github.com/Aydexxx/slipstream/compare/v0.9.0...HEAD
[0.9.0]: https://github.com/Aydexxx/slipstream/releases/tag/v0.9.0
