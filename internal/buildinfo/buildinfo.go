package buildinfo

import "fmt"

// Injected via -ldflags at release build time.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns a human-readable version line.
func String() string {
	return fmt.Sprintf("planfix %s (commit %s, built %s)", Version, Commit, Date)
}
