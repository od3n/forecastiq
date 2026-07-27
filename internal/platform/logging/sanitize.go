package logging

import (
	"context"
	"log/slog"
	"strings"
)

// sensitiveKeys lists field-name patterns that must never appear in log output
// with real values. Matching is done against the lowercased key with `_` and
// `-` separators stripped, so snake_case, camelCase, kebab-case, and
// concatenated variants (api_key / apiKey / api-key / apikey) all match
// (DRB-WP22-010). If any pattern matches, the SanitizingHandler replaces the
// value with "[REDACTED]".
var sensitiveKeys = map[string]bool{
	"token":         true, // also covers refresh_token / access_token / authToken
	"password":      true,
	"passwd":        true,
	"apikey":        true,
	"secret":        true,
	"credential":    true,
	"authorization": true,
	"servicerole":   true,
	"jwt":           true,
	"bearer":        true,
}

const redactedValue = "[REDACTED]"

// SanitizingHandler wraps an slog.Handler and redacts sensitive attribute values
// as a defense-in-depth measure (observability architecture §2: "Never logged").
// This is the last-resort guard; call sites should never log sensitive fields,
// but if they do, this handler prevents them from reaching the output.
type SanitizingHandler struct {
	inner slog.Handler
}

// NewSanitizingHandler wraps the given handler with sensitive-field redaction.
func NewSanitizingHandler(inner slog.Handler) *SanitizingHandler {
	return &SanitizingHandler{inner: inner}
}

// Enabled delegates to the inner handler.
func (h *SanitizingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle sanitizes attrs before passing to the inner handler.
func (h *SanitizingHandler) Handle(ctx context.Context, r slog.Record) error {
	sanitized := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		sanitized.AddAttrs(sanitizeAttr(a))
		return true
	})
	return h.inner.Handle(ctx, sanitized)
}

// WithAttrs sanitizes attrs added via With.
func (h *SanitizingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	sanitized := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		sanitized[i] = sanitizeAttr(a)
	}
	return &SanitizingHandler{inner: h.inner.WithAttrs(sanitized)}
}

// WithGroup delegates to the inner handler.
func (h *SanitizingHandler) WithGroup(name string) slog.Handler {
	return &SanitizingHandler{inner: h.inner.WithGroup(name)}
}

// sanitizeAttr redacts the value if the key is sensitive.
func sanitizeAttr(a slog.Attr) slog.Attr {
	if isSensitiveKey(a.Key) {
		return slog.String(a.Key, redactedValue)
	}
	// Recurse into groups.
	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		sanitized := make([]slog.Attr, len(attrs))
		for i, ga := range attrs {
			sanitized[i] = sanitizeAttr(ga)
		}
		return slog.Group(a.Key, attrsToAny(sanitized)...)
	}
	return a
}

// isSensitiveKey checks if a key matches any known sensitive pattern. The key
// is lowercased and stripped of `_`/`-` separators before the substring match,
// so compound and camelCase keys ("x_api_key", "apiKey", "refreshToken")
// cannot bypass redaction.
func isSensitiveKey(key string) bool {
	normalized := strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(key))
	for sensitive := range sensitiveKeys {
		if strings.Contains(normalized, sensitive) {
			return true
		}
	}
	return false
}

// attrsToAny converts a slice of slog.Attr to []any for slog.Group.
func attrsToAny(attrs []slog.Attr) []any {
	out := make([]any, len(attrs))
	for i, a := range attrs {
		out[i] = a
	}
	return out
}
