# Slipstream

Slipstream is a Windows anti-censorship client built around a single purpose:

- **Fast Mode** — defeats DPI-based blocking (TLS ClientHello fragmentation via
  `winws.exe`/WinDivert) and switches DNS to encrypted Cloudflare DoH. Traffic
  still goes straight out — no proxy, no remote server, no measurable speed
  loss.

It runs from a single window or the system tray. See
[`SECURITY.md`](SECURITY.md) for exactly what Slipstream does to your network
and what it stores.

## Install

Download the latest release from the
[Releases page](https://github.com/Aydexxx/slipstream/releases) — two options:

- **`SlipstreamSetup.exe`** (recommended) — a normal Windows installer. Run
  it, approve the UAC prompt (Slipstream requires Administrator — see
  [`SECURITY.md` › Elevation](SECURITY.md#elevation) for why), and follow the
  wizard. You'll be offered a desktop shortcut and the option to launch
  Slipstream automatically at sign-in.
- **`slipstream.exe`** (portable) — no installation. Download and run it
  directly from any folder; it still requires Administrator and will prompt
  for elevation each time you launch it.

Both are built from the same source by the same script
([`build.ps1`](build.ps1)) — the portable exe is not a stripped-down variant.

## First run

1. **UAC prompt.** Slipstream always runs elevated — it loads a kernel driver
   (WinDivert) and changes the system DNS resolver, neither of which is
   possible without Administrator.
2. On first launch, Slipstream extracts and SHA-256-verifies its bundled
   engine binaries to `%LocalAppData%\Slipstream\engine\`. This only happens
   once (or again after an update).
3. Turn Fast Mode on from the main window or the tray icon — choose Full,
   Discord, or Custom domains and start it. It works immediately; there's
   nothing to configure first.
4. Everything you'd want day-to-day — the on/off toggle, connection status,
   auto-start, "Reset & Quit" — is reachable from the tray icon without
   opening the window.

## Uninstall

Two ways to get to the same result — a full removal, including registry
entries, driver/service artifacts, and everything under
`%LocalAppData%\Slipstream`:

- If you installed via `SlipstreamSetup.exe`: use **Settings → Apps** (or
  **Add or Remove Programs**) and uninstall Slipstream normally.
- Either way: open Slipstream → **Settings → Advanced → Uninstall**. This
  works whether you installed or are running the portable exe, and is what
  the Add/Remove Programs uninstaller calls internally too.

Either path restores your original DNS state and leaves no
trace. To verify this yourself, follow
[`docs/UNINSTALL-VERIFICATION.md`](docs/UNINSTALL-VERIFICATION.md).

**Reset & Quit** (Settings → Advanced, or the tray menu) is the non-destructive
sibling of Uninstall — it restores your network state and closes the app
without removing anything.

## Building from source

Prerequisites: [Go](https://go.dev/), [Node.js](https://nodejs.org/), and the
[Wails CLI](https://wails.io/docs/gettingstarted/installation) (`go install
github.com/wailsapp/wails/v2/cmd/wails@latest`).

```powershell
.\build.ps1
```

This stamps the version from [`VERSION`](VERSION) into the app (in-app, in the
exe's file properties, and in the installer), runs `wails build`, and
produces:

- `dist\slipstream.exe` — the portable exe.
- `dist\SlipstreamSetup.exe` — the installer, if
  [Inno Setup](https://jrsoftware.org/isdl.php) is installed
  (`winget install JRSoftware.InnoSetup`). If it isn't, the script prints
  install instructions and still produces the portable exe — the installer is
  best-effort, not a hard requirement.

### Code signing

`build.ps1` looks for a code-signing certificate in the current user's or
local machine's certificate store and signs both outputs automatically if one
is found; otherwise it skips signing and says so. **Unsigned builds will
trigger Windows SmartScreen** ("Windows protected your PC") on first run,
since there's no publisher reputation to vouch for the binary — this is
expected and not a sign of a broken build. Users can click "More info" → "Run
anyway", but a real code-signing certificate (removing this warning
entirely) is the intended long-term fix.

## Development

Run `wails dev` for hot-reload frontend development (a Vite dev server with
access to the bound Go methods via the browser devtools at
`http://localhost:34115`).

## Documentation

- [`SECURITY.md`](SECURITY.md) — threat model, network behavior, data-at-rest,
  elevation, and the uninstall guarantee.
- [`CHANGELOG.md`](CHANGELOG.md) — release history.
- [`ENGINES.md`](ENGINES.md) — provenance of the bundled third-party binaries.
