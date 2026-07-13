# End-to-end release testing

Four scenarios, each requiring real DPI/a real reboot/a real crash — none of
which can be safely or completely automated. This is the actual gate before
tagging a release: every scenario below must pass, with **no dirty network
state** left behind. "Dirty state" is checked the same way as
[`UNINSTALL-VERIFICATION.md`](UNINSTALL-VERIFICATION.md): a before/after
snapshot of DNS, services, and the registry, not a vibe.

Run everything elevated (Slipstream requires Administrator). Keep
`%LocalAppData%\Slipstream\logs\slipstream.log` open in a tail (`Get-Content
-Wait`) while testing — every state transition and teardown step logs.

---

## 1. Fast Mode on a real blocked site

**Needs**: a site actually subject to DPI-based blocking in your network (or
simulate locally with `tools/fastmode-smoketest`, see below, which proves the
mechanism without needing a censored network).

1. Baseline: `netsh interface ipv4 show dnsservers` and try loading the
   blocked site in a browser — confirm it's actually blocked first (if it
   loads fine, this test proves nothing).
2. Start Fast Mode (Full or the relevant preset/Custom domain).
3. Confirm: `netsh interface ipv4 show dnsservers` now shows `1.1.1.1`/
   `1.0.0.1`; the previously-blocked site now loads; the tray icon and main
   window both show "Fast Mode active".
4. Turn Fast Mode off.
5. Confirm: `netsh interface ipv4 show dnsservers` is back to the **exact**
   baseline from step 1 (same servers, same DHCP/static mode). If it isn't,
   stop and file a bug — do not proceed to other scenarios with dirty DNS.

**For a same-machine mechanism check without a censored network**, run
[`tools/fastmode-smoketest`](../tools/fastmode-smoketest/main.go) — it drives
the real `backend/fastmode` package directly (Start → wait → Stop) and prints
DNS before/during/after, elevated PowerShell required:

```powershell
go run tools/fastmode-smoketest/main.go
```

---

## 2. Rapid on/off toggling

Tests the state machine's mutual-exclusion and teardown-before-next-start
guarantees (`backend/statemachine/manager.go`) under real timing, not just
the fake-driven unit tests.

1. Rapidly click: On → Off → On → Off, without waiting for each to fully
   settle (click the next one as soon as the UI accepts input).
2. Confirm: no click is silently swallowed forever — a request either
   succeeds or surfaces a "still changing mode; try again" toast (never a
   permanently stuck spinner/disabled button).
3. After the sequence settles, confirm final state is clean: if you ended
   "on", Fast Mode is fully and correctly active with `netsh interface ipv4
   show dnsservers` showing `1.1.1.1`; if you ended "off", DNS is fully
   restored to baseline.
4. Turn everything off and re-run the `UNINSTALL-VERIFICATION.md` snapshot
   diff to confirm no residue from the rapid switching.

---

## 3. App crash → clean startup reconciliation

Tests `Manager.Reconcile()` (`backend/statemachine/reconcile.go`), the
crash-safety net every other phase's design depends on.

1. Start Fast Mode. Confirm DNS is hijacked (`netsh interface ipv4 show
   dnsservers` shows `1.1.1.1`).
2. **Hard-kill the app** (not a normal quit):
   ```powershell
   taskkill /IM slipstream.exe /F
   ```
3. Confirm the dirty state persists immediately after the kill (this is
   expected — nothing has reconciled yet): DNS still shows `1.1.1.1`,
   `tasklist | findstr winws` may still show an orphaned process.
4. Relaunch Slipstream. Watch the log for the `Reconcile` sequence
   (`fastmode.KillOrphanedProcesses` → `fastmode.RecoverPendingDNS`).
5. Confirm: DNS is back to your original servers, `tasklist | findstr winws`
   shows nothing, and the app comes up in a clean `Idle` state (not stuck
   showing "Fast Mode active" for a mode that's actually gone).

---

## 4. Reboot with auto-reconnect

Tests `sessionwatch` (session-end handling), autostart, and
`MaybeReconnectLastMode` together — the only scenario that needs an actual
reboot.

1. In Settings, enable **"Resume last mode on launch"** and **"Start with
   Windows"**.
2. Start Fast Mode, confirm it's active.
3. **Reboot the machine** normally (Start menu → Restart, not a hard power
   cycle — this scenario is specifically about the graceful
   `WM_QUERYENDSESSION` path in `backend/sessionwatch`, not crash recovery,
   which scenario 3 already covers).
4. Log back in. Confirm: a UAC prompt appears (autostart launches
   unelevated, then self-elevates — expected, documented in `SECURITY.md`),
   Slipstream starts hidden in the tray (`--autostart` flag suppresses the
   window), and within a few seconds it automatically resumes Fast Mode
   without any click.
5. Before the reboot in step 3, also verify graceful teardown actually ran
   during shutdown: check `slipstream.log` after logging back in for the
   session-end sequence (`sessionwatch` catching `WM_QUERYENDSESSION`,
   `Slipstream is restoring network settings…` block reason, then
   `sm.Shutdown()`), confirming the *previous* session's teardown completed
   before the OS actually powered off — not just that reconnection worked
   afterward.

---

## Pass criteria (all scenarios)

- No scenario leaves DNS or the WinDivert service in a state that doesn't
  match what's currently active (or fully restored, if nothing is active).
- No scenario requires a second app restart or a manual `netsh`/`sc` command
  from you to recover — the app's own reconciliation/teardown logic must do
  it alone.
- Every failure surfaces a clear, actionable message in the UI — never a
  silent hang or a swallowed error.

If everything above passes, proceed to
[`UNINSTALL-VERIFICATION.md`](UNINSTALL-VERIFICATION.md) as the final gate,
then tag the release.
