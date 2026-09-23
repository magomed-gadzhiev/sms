// Package version exposes the release version of the built service binaries.
//
// Version defaults to "dev" for plain `go build` / `go test` and is stamped
// at build time via ldflags (see the Makefile):
//
//	go build -ldflags "-X github.com/smpp-server/smpp-server/internal/version.Version=v0.1.0"
package version

// Version is the release identifier (for example "v0.1.0") of the binary it
// is linked into, or "dev" when built without ldflags stamping.
var Version = "dev"
