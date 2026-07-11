package engine

import "fmt"

// Mode identifies which engine bundle a set of vendored files belongs to.
type Mode string

const (
	ModeFast    Mode = "fastmode"
	ModePrivate Mode = "privatemode"
)

// fileSpec pins one vendored file to its embedded path and expected
// SHA-256. These hashes are the source of truth for verification; see
// ENGINES.md for upstream versions, URLs, and how they were obtained.
type fileSpec struct {
	embedPath string
	sha256    string
}

var fastModeFiles = []fileSpec{
	{embedPath: "bin/fastmode/winws.exe", sha256: "2da71e80878dc270ac83f5893ecbb841f9752a57f1da8ff9325636b4346bc632"},
	{embedPath: "bin/fastmode/WinDivert64.sys", sha256: "8da085332782708d8767bcace5327a6ec7283c17cfb85e40b03cd2323a90ddc2"},
	{embedPath: "bin/fastmode/WinDivert.dll", sha256: "c1e060ee19444a259b2162f8af0f3fe8c4428a1c6f694dce20de194ac8d7d9a2"},
	{embedPath: "bin/fastmode/cygwin1.dll", sha256: "103104a52e5293ce418944725df19e2bf81ad9269b9a120d71d39028e821499b"},
}

var privateModeFiles = []fileSpec{
	{embedPath: "bin/privatemode/amneziawg.exe", sha256: "5475fed5125b13fe7be53b5ee2a6e8b3b8377bac13f983d9cbd6193db989277c"},
	{embedPath: "bin/privatemode/wintun.dll", sha256: "e5da8447dc2c320edc0fc52fa01885c103de8c118481f683643cacc3220dafce"},
}

var allModes = []Mode{ModeFast, ModePrivate}

func filesForMode(mode Mode) ([]fileSpec, error) {
	switch mode {
	case ModeFast:
		return fastModeFiles, nil
	case ModePrivate:
		return privateModeFiles, nil
	default:
		return nil, fmt.Errorf("unknown engine mode %q", mode)
	}
}
