// Package buildinfo exposes version metadata injected at link time:
//
//	go build -ldflags "-X github.com/nuriansyah/hugimethara/internal/platform/buildinfo.Version=v0.1.0 \
//	  -X github.com/nuriansyah/hugimethara/internal/platform/buildinfo.Commit=$(git rev-parse --short HEAD)"
package buildinfo

import "runtime"

var (
	Version = "dev"
	Commit  = "unknown"
)

// GoVersion is the toolchain used to build the binary.
func GoVersion() string { return runtime.Version() }
