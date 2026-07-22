// Package buildinfo exposes build-time metadata injected via -ldflags.
// Values default to "dev" when built without ldflags (e.g. `go run`).
package buildinfo

// Build metadata. Overridden at build time (see Makefile / Dockerfile ldflags).
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// ServiceName is the stable service identifier used in logs and metrics.
const ServiceName = "forecastiq"
