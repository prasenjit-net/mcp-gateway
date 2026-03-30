// Package buildinfo holds build-time metadata injected via ldflags.
package buildinfo

// Version is set by main via ldflags: -X main.version=X.Y.Z
// It defaults to "dev" for local builds.
var Version = "dev"
