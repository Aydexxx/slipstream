<#
.SYNOPSIS
    Builds the Slipstream distributable: stamps the version from VERSION into
    the app, builds it via `wails build`, signs it if a code-signing
    certificate is available (skipped gracefully otherwise), and packages
    dist\slipstream.exe (portable) and dist\SlipstreamSetup.exe (Inno Setup
    installer, if Inno Setup is available).

.EXAMPLE
    .\build.ps1
#>

$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $RepoRoot

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host "==> $Message" -ForegroundColor Cyan
}

function Write-Warn {
    param([string]$Message)
    Write-Host $Message -ForegroundColor Yellow
}

# --- 1. Version -------------------------------------------------------------

$Version = (Get-Content -Path (Join-Path $RepoRoot 'VERSION') -Raw).Trim()
if ($Version -notmatch '^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$') {
    Write-Host "VERSION file does not look like semver: '$Version'" -ForegroundColor Red
    exit 1
}

$GitCommit = 'unknown'
try {
    $rev = git rev-parse --short HEAD 2>$null
    if ($rev) { $GitCommit = $rev.Trim() }
} catch {
    # git not available or not a repo - keep "unknown"
}

$BuildDate = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')

Write-Step "Version $Version (commit $GitCommit, built $BuildDate)"

# --- 2. Stamp wails.json so the exe's Win32 VERSIONINFO/manifest matches ---

Write-Step "Stamping wails.json info.productVersion"
$WailsJsonPath = Join-Path $RepoRoot 'wails.json'
# Explicit UTF8 read: Get-Content's default encoding for a BOM-less file in
# Windows PowerShell 5.1 is not UTF-8, and silently mangles non-ASCII bytes
# (e.g. the (c) in the copyright field) on the read+rewrite round-trip below.
$WailsJsonRaw = [System.IO.File]::ReadAllText($WailsJsonPath, [System.Text.Encoding]::UTF8)
$WailsJson = $WailsJsonRaw | ConvertFrom-Json
if (-not $WailsJson.info) {
    Write-Host "wails.json has no 'info' block to stamp - see build/windows/info.json for what depends on it" -ForegroundColor Red
    exit 1
}
$WailsJson.info.productVersion = $Version
$JsonText = $WailsJson | ConvertTo-Json -Depth 10
[System.IO.File]::WriteAllText($WailsJsonPath, $JsonText + "`n", (New-Object System.Text.UTF8Encoding($false)))

# --- 3. wails build, with the Go-side version vars injected -----------------

Write-Step "Running wails build"
$LdFlags = "-X slipstream/backend/version.Version=$Version -X slipstream/backend/version.GitCommit=$GitCommit -X slipstream/backend/version.BuildDate=$BuildDate"
& wails build -clean -ldflags $LdFlags
if ($LASTEXITCODE -ne 0) {
    Write-Host "wails build failed (exit $LASTEXITCODE)" -ForegroundColor Red
    exit 1
}

$BuiltExe = Join-Path $RepoRoot 'build\bin\slipstream.exe'
if (-not (Test-Path $BuiltExe)) {
    Write-Host "Expected build output not found: $BuiltExe" -ForegroundColor Red
    exit 1
}

# --- 4. Code signing (optional - skipped gracefully with no cert) ----------

Write-Step "Checking for a code-signing certificate"
$Cert = Get-ChildItem Cert:\CurrentUser\My -CodeSigningCert -ErrorAction SilentlyContinue | Select-Object -First 1
if (-not $Cert) {
    $Cert = Get-ChildItem Cert:\LocalMachine\My -CodeSigningCert -ErrorAction SilentlyContinue | Select-Object -First 1
}

$SignToolPath = $null
if ($Cert) {
    $found = Get-Command signtool.exe -ErrorAction SilentlyContinue
    if ($found) {
        $SignToolPath = $found.Source
    } else {
        $kitsRoot = "${env:ProgramFiles(x86)}\Windows Kits\10\bin"
        if (Test-Path $kitsRoot) {
            $candidate = Get-ChildItem $kitsRoot -Recurse -Filter signtool.exe -ErrorAction SilentlyContinue |
                Where-Object { $_.FullName -match '\\x64\\' } |
                Select-Object -First 1
            if ($candidate) { $SignToolPath = $candidate.FullName }
        }
    }
}

$Signed = $false
if ($Cert -and $SignToolPath) {
    Write-Host "Certificate: $($Cert.Subject)"
    Write-Host "signtool:    $SignToolPath"
    & $SignToolPath sign /fd SHA256 /a /tr http://timestamp.digicert.com /td SHA256 $BuiltExe
    if ($LASTEXITCODE -eq 0) {
        $Signed = $true
        Write-Host "Signed build\bin\slipstream.exe"
    } else {
        Write-Warn "Signing failed (signtool exit $LASTEXITCODE) - continuing with an unsigned build."
    }
} else {
    Write-Warn "No code-signing certificate found - producing an UNSIGNED build."
    Write-Warn "See README.md > 'Code signing' for what this means for SmartScreen."
}

# --- 5. dist/ + portable exe --------------------------------------------

Write-Step "Preparing dist\"
$DistDir = Join-Path $RepoRoot 'dist'
if (Test-Path $DistDir) { Remove-Item $DistDir -Recurse -Force }
New-Item -ItemType Directory -Path $DistDir | Out-Null

$PortableExe = Join-Path $DistDir 'slipstream.exe'
Copy-Item $BuiltExe $PortableExe
Write-Host "Portable exe: $PortableExe"

# --- 6. Inno Setup installer (optional - skipped gracefully if missing) ----

Write-Step "Locating Inno Setup (ISCC.exe)"
$Iscc = $null
$found = Get-Command ISCC.exe -ErrorAction SilentlyContinue
if ($found) {
    $Iscc = $found.Source
} else {
    foreach ($candidate in @(
        "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe",
        "${env:ProgramFiles}\Inno Setup 6\ISCC.exe",
        "${env:LOCALAPPDATA}\Programs\Inno Setup 6\ISCC.exe"
    )) {
        if (Test-Path $candidate) { $Iscc = $candidate; break }
    }
}

function Write-Summary {
    Write-Step "Done"
    Write-Host "Version:  $Version"
    Write-Host "Commit:   $GitCommit"
    Write-Host "Signed:   $Signed"
    Write-Host ""
    Get-ChildItem $DistDir | Format-Table Name, Length
}

if (-not $Iscc) {
    Write-Warn ""
    Write-Warn "Inno Setup was not found - skipping the installer build."
    Write-Warn "Install it with:  winget install JRSoftware.InnoSetup"
    Write-Warn "or download from: https://jrsoftware.org/isdl.php"
    Write-Warn "Then re-run this script to also produce dist\SlipstreamSetup.exe."
    Write-Summary
    exit 0
}

Write-Step "Building installer with Inno Setup"
# VersionInfoVersion needs strict N.N.N.N - strip any -prerelease suffix and
# pad to four parts (AppVersion, set separately, keeps the full semver string
# for display).
$VersionNumeric = ($Version -split '-')[0] + '.0'
$IsccArgs = @("/DMyAppVersion=$Version", "/DMyAppVersionNumeric=$VersionNumeric")
if ($Signed -and $SignToolPath) {
    # Best-effort: this path can't be exercised on a machine with no
    # code-signing certificate, so verify the quoting once a real cert is
    # available. Inno Setup substitutes $f for the file being signed.
    $SignCmd = '"' + $SignToolPath + '" sign /fd SHA256 /a /tr http://timestamp.digicert.com /td SHA256 $f'
    $IsccArgs += "/DSIGN=1"
    $IsccArgs += ("/Sslipstreamsign=" + $SignCmd)
}
$IsccArgs += (Join-Path $RepoRoot 'installer\slipstream.iss')

& $Iscc @IsccArgs
if ($LASTEXITCODE -ne 0) {
    Write-Host "Inno Setup compilation failed (exit $LASTEXITCODE)" -ForegroundColor Red
    exit 1
}

$InstallerExe = Join-Path $DistDir 'SlipstreamSetup.exe'
if (-not (Test-Path $InstallerExe)) {
    Write-Host "Expected installer output not found: $InstallerExe" -ForegroundColor Red
    exit 1
}
# No separate signing pass needed here: when $Signed is true, the /DSIGN=1
# define above makes slipstream.iss's SignTool directive active, and Inno
# Setup signs SlipstreamSetup.exe (and its embedded uninstaller, via
# SignedUninstaller=yes) itself as part of compilation.

Write-Summary
