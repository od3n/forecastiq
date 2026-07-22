package ports

import (
	"fmt"
	"time"
)

// ErrorCode is the canonical FC-13 failure classification recorded on every
// non-successful collection. The closed set is authoritative in
// docs/operations/06-provider-failure-runbook.md §1; adapters MUST classify
// into one of these codes so reliability/coverage accounting stays honest.
type ErrorCode string

const (
	// ErrTimeout — upstream did not respond within the deadline (provider side).
	ErrTimeout ErrorCode = "timeout"
	// ErrProvider5xx — provider returned a 5xx after retries (provider side).
	ErrProvider5xx ErrorCode = "provider_5xx"
	// ErrRateLimited — provider returned 429 / signalled budget exhaustion.
	ErrRateLimited ErrorCode = "rate_limited"
	// ErrSchemaDrift — response shape broke the adapter (>50% rows invalid).
	ErrSchemaDrift ErrorCode = "schema_drift"
	// ErrNetworkLocal — our-side network failure reaching the provider.
	ErrNetworkLocal ErrorCode = "network_local"
	// ErrDBError — persistence failure (system side).
	ErrDBError ErrorCode = "db_error"
	// ErrPayloadWriteFailed — raw payload store write failed (system side).
	ErrPayloadWriteFailed ErrorCode = "payload_write_failed"
	// ErrCircuitOpen — breaker refused the call to protect the provider.
	ErrCircuitOpen ErrorCode = "circuit_open"
	// ErrInvalidCredentials — provider rejected the credential (401/403).
	ErrInvalidCredentials ErrorCode = "invalid_credentials"
)

// String returns the wire/DB representation of the code.
func (c ErrorCode) String() string { return string(c) }

// ProviderError is the structured, provider-agnostic classification of a failed
// fetch. Adapters (and the shared transport helper) return it so the collection
// pipeline can account for reliability/coverage and drive retry decisions from
// Retryable rather than re-deriving status codes.
//
// Security: Error never includes response bodies, credentials, URLs, or query
// strings — only the classified code and HTTP status (log-safe).
type ProviderError struct {
	Code       ErrorCode
	HTTPStatus int        // upstream status when known (0 for transport failures)
	Retryable  bool       // FC-08: network/timeout/5xx/429 are retryable
	RateLimit  *RateLimit // populated when the provider signalled limit metadata
	cause      error      // wrapped underlying error (never rendered with body)
}

// NewProviderError builds a classified provider error.
func NewProviderError(code ErrorCode, httpStatus int, retryable bool, cause error) *ProviderError {
	return &ProviderError{Code: code, HTTPStatus: httpStatus, Retryable: retryable, cause: cause}
}

// Error renders a log-safe message: code + status only.
func (e *ProviderError) Error() string {
	if e.HTTPStatus > 0 {
		return fmt.Sprintf("provider error: %s (status %d)", e.Code, e.HTTPStatus)
	}
	return fmt.Sprintf("provider error: %s", e.Code)
}

// Unwrap exposes the underlying cause for errors.Is/As without leaking it into
// the rendered message.
func (e *ProviderError) Unwrap() error { return e.cause }

// Outcome maps the classified code onto the collection Outcome vocabulary.
func (e *ProviderError) Outcome() Outcome {
	switch e.Code {
	case ErrTimeout:
		return OutcomeTimeout
	case ErrRateLimited:
		return OutcomeRateLimited
	case ErrInvalidCredentials:
		return OutcomeAuthFailed
	default:
		return OutcomeFailed
	}
}

// RateLimit is normalized provider rate-limit metadata parsed from standard
// response headers (Retry-After, X-RateLimit-*). All fields are optional; a nil
// *RateLimit means the provider signalled nothing.
type RateLimit struct {
	Limit      int            // request budget for the window (0 = unknown)
	Remaining  int            // requests left in the window (-1 = unknown)
	Reset      *time.Time     // when the window resets (nil = unknown)
	RetryAfter *time.Duration // provider-advised wait before retrying (nil = none)
}
