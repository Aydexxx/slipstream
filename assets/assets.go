// Package assets embeds the vendored third-party engine binaries so they
// ship inside the exe with no runtime downloads. See ENGINES.md for
// provenance (versions, upstream URLs, signature/hash verification notes).
package assets

import "embed"

//go:embed bin/fastmode/winws.exe bin/fastmode/WinDivert64.sys bin/fastmode/WinDivert.dll bin/fastmode/cygwin1.dll
var Bin embed.FS
