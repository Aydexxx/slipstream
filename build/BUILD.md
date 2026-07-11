# Building Slipstream

## Prerequisites

- Go 1.25+
- Node.js 20+ / npm
- Wails v2 CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- Windows 10/11 x64 (WebView2 Runtime, present by default on modern Windows)

## Production build

From the repository root:

```
wails build
```

This generates the Go↔frontend bindings, builds the React/Tailwind frontend
(`frontend/dist`), embeds it into the Go binary, and produces a single
self-contained executable at:

```
build/bin/slipstream.exe
```

The exe carries an embedded manifest requesting `requireAdministrator`
(see `build/windows/wails.exe.manifest`), so Windows/Explorer will prompt
for elevation when it's launched. Double-clicking it (or any launch via
`ShellExecute`, e.g. Explorer or a shortcut) triggers the UAC prompt
automatically because of that manifest. As defense-in-depth for launch
paths that skip manifest processing (`go run`, `wails dev`), the app also
self-checks elevation at startup and relaunches itself elevated if needed
(`backend/elevate`).

Optional flags:

- `-clean` — wipe `build/bin` before building
- `-upx` — compress the binary with UPX (requires UPX installed)
- `-nsis` — also produce a Windows installer (requires NSIS installed)

## Development mode

```
wails dev
```

Runs the app with hot-reload for the frontend. Note the exe produced by
`wails dev` is not manifest-embedded the same way as `wails build` output,
so it typically runs non-elevated — rely on the app's own elevation
self-check in this mode.

## Logs

Structured JSON logs (rotated via lumberjack) are written to:

```
%LocalAppData%\Slipstream\logs\slipstream.log
```

## Single-instance behavior

The app acquires a named global mutex on startup. If another instance is
already running, the new process shows a native "already running" dialog
and exits without opening a second window.
