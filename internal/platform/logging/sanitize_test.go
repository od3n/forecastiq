package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizingHandler_RedactsSensitiveKeys(t *testing.T) {
	tests := []struct {
		key      string
		value    string
		redacted bool
	}{
		{"token", "abc123", true},
		{"password", "secret!", true},
		{"api_key", "sk-123", true},
		{"secret", "hidden", true},
		{"credential", "cred-val", true},
		{"authorization", "Bearer xyz", true},
		{"refresh_token", "rt-abc", true},
		{"access_token", "at-abc", true},
		{"service_role", "role-key", true},
		// Case insensitive
		{"API_KEY", "sk-456", true},
		{"Password", "pass", true},
		// Compound keys containing sensitive words
		{"x_api_key_header", "header-xyz-999", true},
		{"my_secret_value", "ultra-hidden-42", true},
		// Safe keys should not be redacted
		{"provider", "openmeteo", false},
		{"location_id", "uuid-123", false},
		{"duration_ms", "42", false},
		{"status", "success", false},
		{"msg", "collection.completed", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			var buf bytes.Buffer
			inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
			handler := NewSanitizingHandler(inner)
			logger := slog.New(handler)

			logger.Info("test", slog.String(tt.key, tt.value))

			output := buf.String()
			if tt.redacted {
				assert.Contains(t, output, redactedValue,
					"key %q should be redacted in output", tt.key)
				assert.NotContains(t, output, tt.value,
					"original value should not appear for key %q", tt.key)
			} else {
				assert.Contains(t, output, tt.value,
					"key %q should preserve its value", tt.key)
				assert.NotContains(t, output, redactedValue,
					"key %q should not be redacted", tt.key)
			}
		})
	}
}

func TestSanitizingHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := NewSanitizingHandler(inner)
	logger := slog.New(handler).With(slog.String("api_key", "should-be-redacted"))

	logger.Info("event")

	output := buf.String()
	assert.Contains(t, output, redactedValue)
	assert.NotContains(t, output, "should-be-redacted")
}

func TestSanitizingHandler_GroupAttrs(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := NewSanitizingHandler(inner)
	logger := slog.New(handler)

	logger.Info("test", slog.Group("auth",
		slog.String("token", "secret-token"),
		slog.String("user", "alice"),
	))

	output := buf.String()
	assert.Contains(t, output, redactedValue)
	assert.NotContains(t, output, "secret-token")
	assert.Contains(t, output, "alice")
}

func TestSanitizingHandler_Enabled(t *testing.T) {
	inner := slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn})
	handler := NewSanitizingHandler(inner)

	assert.False(t, handler.Enabled(context.Background(), slog.LevelInfo))
	assert.True(t, handler.Enabled(context.Background(), slog.LevelWarn))
	assert.True(t, handler.Enabled(context.Background(), slog.LevelError))
}

func TestIsSensitiveKey(t *testing.T) {
	positives := []string{"token", "TOKEN", "api_key", "my_api_key_value",
		"password", "secret", "credential", "authorization", "refresh_token",
		"access_token", "service_role",
		// camelCase / kebab-case / concatenated variants (DRB-WP22-010).
		"apiKey", "api-key", "authToken", "refreshToken", "passwd", "jwt",
		"bearer", "serviceRole"}
	for _, k := range positives {
		assert.True(t, isSensitiveKey(k), "expected %q to be sensitive", k)
	}

	negatives := []string{"provider", "location_id", "status", "msg", "level",
		"duration_ms", "snapshots_stored", "error_code"}
	for _, k := range negatives {
		assert.False(t, isSensitiveKey(k), "expected %q to NOT be sensitive", k)
	}
}

func TestEventConstants_AreStable(t *testing.T) {
	// Verify a sampling of event constants have the expected dot-namespaced format.
	events := []string{
		EventCollectionStarted, EventCollectionCompleted, EventCollectionFailed,
		EventObservationCollected, EventMatchingBatchCompleted, EventRankingsPublished,
		EventSchedulerSlotClaimed, EventCircuitOpened, EventAuthLoginFailed,
	}
	for _, e := range events {
		require.True(t, strings.Contains(e, "."), "event %q must be dot-namespaced", e)
	}
}

// TestStructuredLogOutput_BindingFields verifies that the logging pipeline
// produces JSON with the required binding fields per observability architecture §2.
func TestStructuredLogOutput_BindingFields(t *testing.T) {
	var buf bytes.Buffer
	logger := New("info", "json", &buf)

	logger.Info(EventCollectionCompleted,
		slog.String("provider", "openmeteo"),
		slog.String("collection_id", "test-id-123"),
		slog.Int("duration_ms", 812),
	)

	output := buf.String()

	// Required binding fields (architecture §2).
	assert.Contains(t, output, `"level"`, "must contain level field")
	assert.Contains(t, output, `"msg"`, "must contain msg field")
	assert.Contains(t, output, `"service"`, "must contain service field")
	assert.Contains(t, output, `"time"`, "must contain timestamp field")

	// Event name should be the msg value.
	assert.Contains(t, output, EventCollectionCompleted)

	// Context fields present.
	assert.Contains(t, output, `"provider"`, "must contain provider context")
	assert.Contains(t, output, `"collection_id"`, "must contain collection_id context")
	assert.Contains(t, output, `"duration_ms"`, "must contain duration_ms context")

	// Service name binding (from buildinfo).
	assert.Contains(t, output, "forecastiq")
}

// TestStructuredLogOutput_NoSecretPatterns verifies that logging with the
// sanitizing handler does not leak sensitive patterns in output.
func TestStructuredLogOutput_NoSecretPatterns(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := NewSanitizingHandler(inner)
	logger := slog.New(handler)

	// Simulate a badly-written log call that includes sensitive data.
	logger.Info("auth.attempt",
		slog.String("user", "alice"),
		slog.String("token", "eyJhbGciOiJSUzI1NiJ9.secret"),
		slog.String("api_key", "sk_live_1234567890"),
		slog.String("password", "hunter2"),
	)

	output := buf.String()

	// Sensitive values must not appear.
	secretPatterns := []string{
		"eyJhbGciOiJSUzI1NiJ9",
		"sk_live_1234567890",
		"hunter2",
	}
	for _, pat := range secretPatterns {
		assert.NotContains(t, output, pat, "secret pattern %q leaked into log output", pat)
	}

	// Safe values should still be present.
	assert.Contains(t, output, "alice")
	assert.Contains(t, output, redactedValue)
}
