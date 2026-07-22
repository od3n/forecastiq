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
