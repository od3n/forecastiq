package collection

import (
	"context"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/collection/domain"
	"github.com/forecastiq/forecastiq/internal/collection/ports"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
	"github.com/forecastiq/forecastiq/internal/platform/ids"
)

// ReaderService implements ForecastReader (admin lineage query + latest
// forecast read). Read endpoints use the pool directly (no transaction).
type ReaderService struct {
	collections ports.CollectionRepository
	snapshots   ports.SnapshotRepository
	pool        dbtx.DBTX
}

// NewReaderService wires a ReaderService.
func NewReaderService(collections ports.CollectionRepository, snapshots ports.SnapshotRepository, pool dbtx.DBTX) *ReaderService {
	return &ReaderService{collections: collections, snapshots: snapshots, pool: pool}
}

// GetCollection returns a collection by id (404 when unknown).
func (s *ReaderService) GetCollection(ctx context.Context, id uuid.UUID) (*domain.ForecastCollection, error) {
	return s.collections.GetByID(ctx, s.pool, id)
}

// ListCollections returns a keyset-paginated collection page (admin).
func (s *ReaderService) ListCollections(ctx context.Context, in CollectionListInput) ([]*domain.ForecastCollection, PageInfo, error) {
	limit := in.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var cursor uuid.UUID
	if in.Cursor != "" {
		parsed, err := ids.Parse(in.Cursor)
		if err != nil {
			return nil, PageInfo{}, &validationError{field: "cursor", message: "must be a valid UUID"}
		}
		cursor = parsed
	}
	rows, err := s.collections.List(ctx, s.pool, ports.CollectionFilter{
		ProviderID: in.ProviderID, LocationID: in.LocationID, Status: in.Status,
		Cursor: cursor, Limit: limit,
	})
	if err != nil {
		return nil, PageInfo{}, err
	}
	page := PageInfo{}
	if len(rows) > limit {
		rows = rows[:limit]
		page.HasMore = true
	}
	if page.HasMore && len(rows) > 0 {
		page.NextCursor = rows[len(rows)-1].ID.String()
	}
	return rows, page, nil
}

// LatestForecast returns the most recent successful collection + snapshots for
// a provider+location, or nil when none exists.
func (s *ReaderService) LatestForecast(ctx context.Context, providerID, locationID uuid.UUID) (*LatestForecast, error) {
	coll, err := s.collections.LatestSuccessful(ctx, s.pool, providerID, locationID)
	if err != nil {
		return nil, err
	}
	if coll == nil {
		return nil, nil
	}
	snaps, err := s.snapshots.ByCollectionID(ctx, s.pool, coll.ID)
	if err != nil {
		return nil, err
	}
	return &LatestForecast{Collection: coll, Snapshots: snaps}, nil
}

// SnapshotsByCollection returns all snapshots for a collection.
func (s *ReaderService) SnapshotsByCollection(ctx context.Context, collectionID uuid.UUID) ([]*domain.ForecastSnapshot, error) {
	return s.snapshots.ByCollectionID(ctx, s.pool, collectionID)
}

// ProviderLineages returns the per-provider public lineage projection keyed by
// provider id (adapter_version of the latest successful collection +
// collecting_since). Providers with no collections are absent from the map.
func (s *ReaderService) ProviderLineages(ctx context.Context) (map[uuid.UUID]ProviderLineage, error) {
	rows, err := s.collections.ProviderLineages(ctx, s.pool)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]ProviderLineage, len(rows))
	for _, l := range rows {
		out[l.ProviderID] = ProviderLineage{AdapterVersion: l.AdapterVersion, CollectingSince: l.CollectingSince}
	}
	return out, nil
}

// GetSnapshot returns a snapshot by id (404 when unknown).
func (s *ReaderService) GetSnapshot(ctx context.Context, id uuid.UUID) (*domain.ForecastSnapshot, error) {
	return s.snapshots.GetByID(ctx, s.pool, id)
}

// validationError is a minimal field error for reader query validation.
type validationError struct {
	field   string
	message string
}

func (e *validationError) Error() string { return e.field + ": " + e.message }

// Field returns the offending field (consumed by the API error mapper).
func (e *validationError) Field() string { return e.field }

// Message returns the constraint message.
func (e *validationError) Message() string { return e.message }
