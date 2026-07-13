# Vendored Engine Binaries

Slipstream bundles the third-party binaries it drives directly inside the
exe (`go:embed`, see `assets/assets.go`). Nothing is downloaded at runtime —
everything below was fetched once, verified against upstream, and committed
as-is under `assets/bin/`. The SHA-256 values here are duplicated as
hardcoded constants in `backend/engine/manifest.go`; that copy, not this
file, is what the app actually checks at runtime.

All files are Windows x64 (`windows-x86_64` / `amd64`) only, matching the
app's target platform.

## Fast Mode — zapret

Upstream: https://github.com/bol-van/zapret
Release: `v72.12` (https://github.com/bol-van/zapret/releases/tag/v72.12)
Source archive: https://github.com/bol-van/zapret/releases/download/v72.12/zapret-v72.12.zip
Files taken from `zapret-v72.12/binaries/windows-x86_64/` inside that archive.

zapret publishes a `sha256sum.txt` in the same release covering every file
in the archive; the hashes below were checked against it at vendor time.

| File | SHA-256 | Notes |
|---|---|---|
| `assets/bin/fastmode/winws.exe` | `2da71e80878dc270ac83f5893ecbb841f9752a57f1da8ff9325636b4346bc632` | Main zapret engine (Cygwin-linked). Not Authenticode-signed upstream. |
| `assets/bin/fastmode/WinDivert64.sys` | `8da085332782708d8767bcace5327a6ec7283c17cfb85e40b03cd2323a90ddc2` | Kernel packet-filter driver. Authenticode-signed (see caveat below). |
| `assets/bin/fastmode/WinDivert.dll` | `c1e060ee19444a259b2162f8af0f3fe8c4428a1c6f694dce20de194ac8d7d9a2` | Userspace loader for the driver. Not signed upstream. |
| `assets/bin/fastmode/cygwin1.dll` | `103104a52e5293ce418944725df19e2bf81ad9269b9a120d71d39028e821499b` | Cygwin runtime `winws.exe` requires to run. Not signed upstream. |

**Provenance note:** `winws.exe`, `WinDivert.dll`, and `cygwin1.dll` in the
zapret release carry no Authenticode signature — this is normal for the
zapret project and doesn't affect their ability to run (they're userspace
binaries, not drivers). `WinDivert64.sys` *is* Authenticode-signed since
Windows requires kernel drivers to be signed to load, but the signing
certificate belongs to a third-party entity ("成都密思听科技有限公司",
Chengdu, CN) rather than WinDivert's original author (basil00) or
reqrypt.org directly — this is the zapret project's own re-signed build of
WinDivert, bundled specifically for ABI compatibility with their `winws.exe`
build. We deliberately took WinDivert from this same zapret release rather
than a separate basil00/WinDivert release, to avoid pairing a `winws.exe`
with a WinDivert version it wasn't built/tested against.

Trust basis: HTTPS download from the project's official GitHub releases,
hash-checked against the project's own published `sha256sum.txt`. This is
self-consistency (confirms the download wasn't corrupted/tampered in
transit and matches what the project's CI produced), not independent
third-party attestation — zapret does not GPG-sign releases.

## Re-vendoring / updating a pinned version

1. Download the new release from the URLs above (or their successors).
2. Verify it the same way this file documents (upstream hash manifest
   and/or Authenticode signature) — don't skip this step.
3. Replace the file(s) under `assets/bin/...`.
4. Update the SHA-256 hardcoded in `backend/engine/manifest.go`.
5. Update the tables and version/URL references in this file.
6. Rebuild — mismatched files extracted from an old install will be
   detected and re-extracted automatically, but bumping the version is a
   deliberate, reviewed action, not an automatic one.
