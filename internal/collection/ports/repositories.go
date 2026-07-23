package ports

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/collection/domain"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// CollectionFilter selects collections for the admin lineage query.
type CollectionFilter struct {
	ProviderID *uuid.UUID
	LocationID *uuid.UUID
	Status     *domain.CollectionStatus
	Cursor     uuid.UUID
	Limit      int
}

// CollectionRepository persists ForecastCollection aggregates.
type CollectionRepository interface {
	// Insert writes the collection row (typically status=pending).
	Insert(ctx context.Context, tx dbtx.DBTX, c *domain.ForecastCollection) error
	// Complete performs the single final UPDATE to a terminal status + counts
	// (permitted only while the row is still pending; immutability thereafter).
	Complete(ctx context.Context, tx dbtx.DBTX, c *domain.ForecastCollection) error
	GetByID(ctx context.Context, tx dbtx.DBTX, id uuid.UUID) (*domain.ForecastCollection, error)
	// FindDedup returns an existing success/partial collection matching the
	// collection-level dedup key, or nil (domain §4.3).
	FindDedup(ctx context.Context, tx dbtx.DBTX, providerID, locationID uuid.UUID, dedupKey time.Time) (*domain.ForecastCollection, error)
	// LatestSuccessful returns the most recent success/partial collection for
	// a provider+location, or nil.
	LatestSuccessful(ctx context.Context, tx dbtx.DBTX, providerID, locationID uuid.UUID) (*domain.ForecastCollection, error)
	// List returns up to filter.Limit+1 rows (extra detects has_more).
	List(ctx context.Context, tx dbtx.DBTX, f CollectionFilter) ([]*domain.ForecastCollection, error)
}

// SnapshotRepository persists immutable ForecastSnapshot children.
type SnapshotRepository interface {
	// EnsurePartitions creates any missing monthly partitions covering the
	// given month-start instants (idempotent DDL; run before inserts).
	EnsurePartitions(ctx context.Context, tx dbtx.DBTX, monthStarts []time.Time) error
	// InsertBatch inserts snapshots with ON CONFLICT DO NOTHING and returns
	// the number of rows actually stored (dedup boundary; domain §4.3).
	InsertBatch(ctx context.Context, tx dbtx.DBTX, snapshots []*domain.ForecastSnapshot) (stored int, err error)
	ByCollectionID(ctx context.Context, tx dbtx.DBTX, collectionID uuid.UUID) ([]*domain.ForecastSnapshot, error)
	GetByID(ctx context.Context, tx dbtx.DBTX, id uuid.UUID) (*domain.ForecastSnapshot, error)
}

// ObservationRepository persists observation rows (WP-10). Observations have no
// parent collection entity (ADR-025); dedup and supersession are enforced at
// the row level. The live-row dedup boundary is (source, location_id,
// observed_at) among non-superseded rows.
type ObservationRepository interface {
	// EnsurePartitions creates any missing monthly partitions covering the
	// given month-start instants (idempotent DDL; run before inserts).
	EnsurePartitions(ctx context.Context, tx dbtx.DBTX, monthStarts []time.Time) error
	// ListCurrentByWindow returns the non-superseded observation rows for a
	// (source, location) whose observed_at is within [start, end], keyed for
	// correction comparison (workflow §4).
	ListCurrentByWindow(ctx context.Context, tx dbtx.DBTX, source string, locationID uuid.UUID, start, end time.Time) ([]*domain.Observation, error)
	// InsertBatch inserts observation rows with ON CONFLICT DO NOTHING against
	// the live-row dedup boundary and returns the number actually stored.
	InsertBatch(ctx context.Context, tx dbtx.DBTX, obs []*domain.Observation) (stored int, err error)
	// Supersede sets superseded_observation_id on the old row (the only
	// permitted observation mutation; domain §2.7). Must run before inserting
	// the correcting row so the live-row dedup index does not conflict.
	Supersede(ctx context.Context, tx dbtx.DBTX, oldID uuid.UUID, oldObservedAt time.Time, newID uuid.UUID) error
	// LatestObservedAt returns the most recent observed_at for a
	// (source, location), or ok=false when none exists (freshness gauge).
	LatestObservedAt(ctx context.Context, tx dbtx.DBTX, source string, locationID uuid.UUID) (t time.Time, ok bool, err error)
}
