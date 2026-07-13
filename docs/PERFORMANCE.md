# Performance check

Methodology for confirming Fast Mode has negligible speed/ping impact — it
requires a live network and Fast Mode actually enabled, so it's a manual check
using [`tools/perfcheck.ps1`](../tools/perfcheck.ps1), not an automated test.

## Why this needs to be live

- **Fast Mode** adds TLS ClientHello fragmentation (WinDivert intercepting
  and rewriting a handful of packets per new connection) and switches DNS to
  Cloudflare DoH. Neither should add meaningful per-packet latency once a
  connection is established — the expected result is "no measurable
  difference" — but that's an empirical claim, not something provable from
  reading the code.

## Procedure

Run elevated, with nothing else saturating your connection.

1. **Baseline** (Fast Mode off):
   ```powershell
   .\tools\perfcheck.ps1 -Label baseline
   ```
2. **Fast Mode**: start it in the app, wait a few seconds for DNS to settle,
   then:
   ```powershell
   .\tools\perfcheck.ps1 -Label fastmode-on
   ```
3. Compare the two `%TEMP%\slipstream-perf-*.txt` reports:
   ```powershell
   Get-Content $env:TEMP\slipstream-perf-baseline.txt
   Get-Content $env:TEMP\slipstream-perf-fastmode-on.txt
   ```

## What to look for

- **Fast Mode ping delta**: should be within normal network jitter of
  baseline (a few ms at most). A consistent, large increase (tens of ms or
  more) would indicate the WinDivert packet interception path is adding
  real overhead and is worth investigating before release.
- **Fast Mode throughput delta**: should be within measurement noise of
  baseline (no proxying/re-routing happens in Fast Mode — it's DPI evasion
  plus a DNS change, and traffic still goes straight out).

There's no fixed pass/fail threshold here (it depends on your baseline
connection), but record the numbers alongside the release so future
comparisons have something to regress against.
