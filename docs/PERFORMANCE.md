# Performance check

Methodology for confirming Fast Mode has negligible speed/ping impact and
measuring Private Mode's overhead — both require a live network and a mode
actually enabled, so they're a manual check using
[`tools/perfcheck.ps1`](../tools/perfcheck.ps1), not an automated test.

## Why these need to be live

- **Fast Mode** adds TLS ClientHello fragmentation (WinDivert intercepting
  and rewriting a handful of packets per new connection) and switches DNS to
  Cloudflare DoH. Neither should add meaningful per-packet latency once a
  connection is established — the expected result is "no measurable
  difference" — but that's an empirical claim, not something provable from
  reading the code.
- **Private Mode** routes all traffic through a full WireGuard (AmneziaWG)
  tunnel with obfuscation to *your* VPS — this inherently adds one extra hop
  (real RTT to your VPS) plus encryption/obfuscation CPU overhead. Some
  overhead here is expected and correct; the check is about **quantifying**
  it, not proving it's zero.

## Procedure

Run elevated, with nothing else saturating your connection.

1. **Baseline** (both modes off):
   ```powershell
   .\tools\perfcheck.ps1 -Label baseline
   ```
2. **Fast Mode**: start it in the app, wait a few seconds for DNS to settle,
   then:
   ```powershell
   .\tools\perfcheck.ps1 -Label fastmode-on
   ```
   Turn Fast Mode off before continuing.
3. **Private Mode**: connect, wait for `Connected` with a fresh handshake,
   then:
   ```powershell
   .\tools\perfcheck.ps1 -Label privatemode-on
   ```
4. Compare the three `%TEMP%\slipstream-perf-*.txt` reports:
   ```powershell
   Get-Content $env:TEMP\slipstream-perf-baseline.txt
   Get-Content $env:TEMP\slipstream-perf-fastmode-on.txt
   Get-Content $env:TEMP\slipstream-perf-privatemode-on.txt
   ```

## What to look for

- **Fast Mode ping delta**: should be within normal network jitter of
  baseline (a few ms at most). A consistent, large increase (tens of ms or
  more) would indicate the WinDivert packet interception path is adding
  real overhead and is worth investigating before release.
- **Fast Mode throughput delta**: should be within measurement noise of
  baseline (no proxying/re-routing happens in Fast Mode — it's DPI evasion
  plus a DNS change, not a tunnel).
- **Private Mode ping delta**: expected to be higher than baseline — the
  floor is your real RTT to your VPS, plus a small constant for
  encryption/obfuscation. Sanity-check it's in the ballpark of a plain `ping
  <your-VPS-IP>` from the same machine, not wildly higher (which would
  suggest the obfuscation parameters or routing are doing something
  expensive).
- **Private Mode throughput delta**: expected to be somewhat lower than
  baseline (WireGuard + obfuscation overhead, plus your VPS's own uplink
  capacity becomes the ceiling). A drop to a small fraction of baseline
  suggests a VPS-side bottleneck (undersized instance, saturated uplink) more
  than an app-side problem — cross-check against your VPS's own bandwidth
  before treating it as a Slipstream regression.

There's no fixed pass/fail threshold here (it depends on your baseline
connection and VPS), but record the numbers alongside the release so future
comparisons have something to regress against.
