// Package collection is the collection module's application layer: the
// idempotent forecast-collection use case (provider call → validate →
// decompose → store with lineage), the read model, and the interfaces other
// modules consume (ForecastCollector, ForecastReader — module architecture
// §3.3). One bounded transaction per collection (ADR-027).
package collection

import (
	"context"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/catalog"
	"github.com/forecastiq/forecastiq/internal/collection/domain"
)

// Collection source provenance (audit + logs).
const (
	SourceManual    = "manual"
	SourceScheduled = "scheduled"
	SourceReplay    = "replay"
)

// CollectInput is the RunForecastCollection command. The catalog entities are
// resolved + validated (active) by the caller (trigger handler / scheduler).
type CollectInput struct {
	Provider *catalog.Provider
	Location *catalog.Location
	Config   *catalog.ProviderConfiguration
	Actor    catalog.Actor
	Source   string
}

// ForecastCollector runs forecast collections.
type ForecastCollector interface {
	Collect(ctx context.Context, in CollectInput) (*domain.ForecastCollection, error)
}

// ForecastReplayer reprocesses a stored raw payload through the current
// adapter (FC-14; workflow 06 §2). No provider network call occurs; the
// original collection and its snapshots are never mutated.
type ForecastReplayer interface {
	Replay(ctx context.Context, collectionID uuid.UUID, actor catalog.Actor) (*domain.ForecastCollection, error)
}

// CollectionListInput is the admin collection query.
type CollectionListInput struct {
	ProviderID *uuid.UUID
	LocationID *uuid.UUID
	Status     *domain.CollectionStatus
	Cursor     string
	Limit      int
}

// LatestForecast bundles a collection with its snapshots (latest-forecast read).
type LatestForecast struct {
	Collection *domain.ForecastCollection
	Snapshots  []*domain.ForecastSnapshot
}

// ForecastReader is the collection read model.
type ForecastReader interface {
	GetCollection(ctx context.Context, id uuid.UUID) (*domain.ForecastCollection, error)
	ListCollections(ctx context.Context, in CollectionListInput) ([]*domain.ForecastCollection, PageInfo, error)
	LatestForecast(ctx context.Context, providerID, locationID uuid.UUID) (*LatestForecast, error)
	SnapshotsByCollection(ctx context.Context, collectionID uuid.UUID) ([]*domain.ForecastSnapshot, error)
	GetSnapshot(ctx context.Context, id uuid.UUID) (*domain.ForecastSnapshot, error)
}

// PageInfo is a keyset-pagination result.
type PageInfo struct {
	HasMore    bool
	NextCursor string
}
