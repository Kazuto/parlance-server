// Package version tells which release of the server is running.
package version

// Version is the release, e.g. "2.0.0", set when building with
// -ldflags "-X github.com/kazuto/parlance-server/internal/version.Version=2.0.0".
// Builds from source without it are "dev".
var Version = "dev"
