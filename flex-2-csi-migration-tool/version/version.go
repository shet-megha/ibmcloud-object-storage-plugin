package version

import "fmt"

var (
	// Version is the semantic version (set at build time via -ldflags)
	Version = "dev"
	// GitCommit is the git commit hash (set at build time via -ldflags)
	GitCommit = "unknown"
	// BuildDate is the build date (set at build time via -ldflags)
	BuildDate = "unknown"
)

// GetVersion returns the version string
func GetVersion() string {
	return fmt.Sprintf("v%s", Version)
}

// GetFullVersion returns the full version string with commit and build date
func GetFullVersion() string {
	return fmt.Sprintf("v%s (commit: %s, built: %s)", Version, GitCommit, BuildDate)
}

// GetVersionNumber returns just the version number without 'v' prefix
func GetVersionNumber() string {
	return Version
}

// Made with Bob
