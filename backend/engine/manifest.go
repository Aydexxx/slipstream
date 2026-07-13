package engine

import "fmt"

// Mode identifies which engine bundle a set of vendored files belongs to.
type Mode string

const (
	ModeFast Mode = "fastmode"
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

var allModes = []Mode{ModeFast}

func filesForMode(mode Mode) ([]fileSpec, error) {
	switch mode {
	case ModeFast:
		return fastModeFiles, nil
	default:
		return nil, fmt.Errorf("unknown engine mode %q", mode)
	}
}
