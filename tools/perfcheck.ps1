<#
.SYNOPSIS
    One-off network performance snapshot: ping latency to a few fixed IPs
    (not hostnames, so DNS state can't confound the measurement) plus a
    download-throughput sample. Run once with a mode off, then again with a
    mode on, and compare the two reports - see docs/PERFORMANCE.md.

    Not part of the build; a manual measurement helper, same category as
    tools/gentrayicons and tools/fastmode-smoketest.

.EXAMPLE
    .\tools\perfcheck.ps1 -Label baseline
    .\tools\perfcheck.ps1 -Label fastmode-on
#>
param(
    [Parameter(Mandatory = $true)][string]$Label,
    [int]$PingCount = 10,
    [int]$DownloadBytes = 10000000  # 10MB
)

$ErrorActionPreference = 'Stop'
$out = "$env:TEMP\slipstream-perf-$Label.txt"
"=== Slipstream performance snapshot: $Label @ $(Get-Date -Format o) ===" | Out-File $out

function Section {
    param([string]$Title, [scriptblock]$Script)
    "`n--- $Title ---" | Out-File $out -Append
    try { & $Script 2>&1 | Out-File $out -Append } catch { "ERROR: $_" | Out-File $out -Append }
}

# Fixed IPs, not hostnames: with Fast Mode on, DNS itself is redirected to
# Cloudflare, so a hostname-based ping would measure DNS behavior as much as
# network latency. Pinging IPs directly isolates the actual RTT delta.
$targets = @(
    @{Name = "Cloudflare"; IP = "1.1.1.1" },
    @{Name = "Google"; IP = "8.8.8.8" }
)

foreach ($t in $targets) {
    Section "Ping: $($t.Name) ($($t.IP))" {
        $results = Test-Connection -TargetName $t.IP -Count $PingCount -ErrorAction SilentlyContinue
        if (-not $results) {
            "no response"
        } else {
            $times = $results | ForEach-Object { $_.Latency }
            $avg = ($times | Measure-Object -Average).Average
            $min = ($times | Measure-Object -Minimum).Minimum
            $max = ($times | Measure-Object -Maximum).Maximum
            "avg={0}ms min={1}ms max={2}ms (n=$PingCount)" -f $avg, $min, $max
        }
    }
}

# Cloudflare's own public speed-test download endpoint (speed.cloudflare.com)
# - the same trust boundary this app already uses for DoH and the external-IP
# trace endpoint (backend/fastmode/dns.go, backend/privatemode/externalip.go).
Section "Download throughput ($([math]::Round($DownloadBytes/1MB, 1)) MB sample)" {
    $url = "https://speed.cloudflare.com/__down?bytes=$DownloadBytes"
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $resp = Invoke-WebRequest -Uri $url -UseBasicParsing
    $sw.Stop()
    $seconds = $sw.Elapsed.TotalSeconds
    $mbps = ($resp.RawContentLength * 8 / 1MB) / $seconds
    "{0:N2} Mbps ({1} bytes in {2:N2}s)" -f $mbps, $resp.RawContentLength, $seconds
}

Write-Host "Report written to $out"
Get-Content $out
