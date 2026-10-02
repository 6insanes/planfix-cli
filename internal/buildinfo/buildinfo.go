package buildinfo

// Injected via -ldflags at release build time.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)
