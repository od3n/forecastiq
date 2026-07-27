// Package logging configures structured JSON logging via log/slog per the
// observability architecture (§2). Binding fields: ts, level, msg, service.
// Context fields (request_id, job_id, collection_id, provider, location_id)
// are attached per request/job. Secrets and provider bodies are never logged.
package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/forecastiq/forecastiq/internal/platform/buildinfo"
)

// ctxKey is the private type for storing a logger in context.
type ctxKey struct{}

// New builds the application logger. format is "json" (production) or
// "text" (local dev). level is one of debug|info|warn|error.
func New(level, format string, w io.Writer) *slog.Logger {
	if w == nil {
		w = os.Stdout
	}
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}
	var handler slog.Handler
	if strings.ToLower(format) == "text" {
		handler = slog.NewTextHandler(w, opts)
	} else {
		handler = slog.NewJSONHandler(w, opts)
	}

	// Defense-in-depth: wrap with the sanitizing handler so sensitive field
	// names are always redacted even if a call site accidentally logs them
	// (observability architecture §2: "Never logged").
	handler = NewSanitizingHandler(handler)

	return slog.New(handler).With(slog.String("service", buildinfo.ServiceName))
}

// WithContext returns a context carrying the given logger.
func WithContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, logger)
}

// FromContext returns the logger stored in ctx, or the supplied fallback
// (typically slog.Default()) when none is present.
func FromContext(ctx context.Context, fallback *slog.Logger) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	if fallback != nil {
		return fallback
	}
	return slog.Default()
}
