// Package version holds build-time version metadata for Slipstream.
// Values are overridden at build time via -ldflags, e.g.:
//
//	-ldflags "-X 'slipstream/backend/version.Version=1.0.0' -X 'slipstream/backend/version.GitCommit=abc123'"
package version

var (
	Version   = "0.1.0-dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)
