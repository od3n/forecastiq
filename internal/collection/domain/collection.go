// Package domain holds the collection module's pure domain model: the
// ForecastCollection aggregate root, its immutable ForecastSnapshot children,
// status lifecycle, and invariants. Infrastructure-free (binding rule).
package domain

import (
	"time"

	"github.com/google/uuid"
)

// CollectionStatus is the lifecycle state of a forecast collection.
// pending → one terminal state; terminal states are final (immutability
// trigger enforced after completion).
type CollectionStatus string

const (
	StatusPending      CollectionStatus = "pending"
	StatusSuccess      CollectionStatus = "success"
	StatusPartial      CollectionStatus = "partial"
	StatusFailed       CollectionStatus = "failed"
	StatusDeduplicated CollectionStatus = "deduplicated"
	StatusRateLimited  CollectionStatus = "rate_limited"
	StatusTimeout      CollectionStatus = "timeout"
)

// Terminal reports whether the status is final.
func (s CollectionStatus) Terminal() bool { return s != StatusPending }

// Successful reports whether the collection produced usable data.
func (s CollectionStatus) Successful() bool {
	return s == StatusSuccess || s == StatusPartial
}

// ForecastCollection is one provider API exchange for one location (parent
// record). Immutable after completion.
type ForecastCollection struct {
	ID                      uuid.UUID
	ProviderID              uuid.UUID
	LocationID              uuid.UUID
	ProviderConfigurationID uuid.UUID
	RequestedAt             time.Time
	CompletedAt             *time.Time
	Status                  CollectionStatus
	ProviderRequestID       string
	ProviderModelRunTime    *time.Time
	RawPayloadObjectKey     string
	RawPayloadChecksum      string
	ResponseStatusCode      int
	ResponseLatencyMS       int
	RecordsReceived         int
	SnapshotsStored         int
	SnapshotsDeduplicated   int
	SnapshotsInvalid        int
	SchemaVersion           string
	AdapterVersion          string
	ErrorCode               string
	ErrorMessage            string
	CreatedAt               time.Time
}

// AccountingHolds verifies records_received = stored + deduplicated + invalid
// (required for success/partial; domain §4.2).
func (c *ForecastCollection) AccountingHolds() bool {
	return c.RecordsReceived == c.SnapshotsStored+c.SnapshotsDeduplicated+c.SnapshotsInvalid
}
