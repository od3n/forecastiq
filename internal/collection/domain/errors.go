package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Sentinel domain errors mapped to RFC 7807 classes by the API layer.
var (
	ErrNotFound           = errors.New("resource not found")
	ErrInactive           = errors.New("provider, location, or configuration is not active")
	ErrPayloadUnavailable = errors.New("raw payload unavailable (expired or corrupt)")
	// ErrReplayUnsupported signals the provider's adapter cannot deterministically
	// decode a stored payload (does not implement ReplayDecoder; FC-14).
	ErrReplayUnsupported = errors.New("provider adapter does not support replay")
	// ErrDuplicateCollection signals a concurrent collection committed the same
	// dedup key first (collection-level dedup race; domain §4.3). The collector
	// records this attempt as a deduplicated collection rather than failing.
	ErrDuplicateCollection = errors.New("duplicate collection (concurrent commit of same dedup key)")
)

// CircuitOpenError signals that the provider circuit is open (409 conflict on
// manual trigger). RetryAt is the next half-open probe time.
type CircuitOpenError struct {
	ProviderID uuid.UUID
	RetryAt    *time.Time
}

// Error implements error.
func (e *CircuitOpenError) Error() string {
	return fmt.Sprintf("provider circuit open (provider=%s); next probe at %v", e.ProviderID, e.RetryAt)
}
