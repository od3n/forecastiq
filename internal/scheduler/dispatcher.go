package scheduler

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/forecastiq/forecastiq/internal/catalog"
	"github.com/forecastiq/forecastiq/internal/collection"
)

// ForecastDispatcher dispatches forecast_collection slots to the collection
// module. It resolves the slot's configuration, provider, and location, then
// runs an idempotent collection (SourceScheduled).
type ForecastDispatcher struct {
	configs   catalog.ConfigurationManager
	providers catalog.ProviderCatalog
	locations catalog.LocationManager
	collector collection.ForecastCollector
	logger    *slog.Logger
}

// NewForecastDispatcher wires a ForecastDispatcher.
func NewForecastDispatcher(configs catalog.ConfigurationManager, providers catalog.ProviderCatalog,
	locations catalog.LocationManager, collector collection.ForecastCollector, logger *slog.Logger) *ForecastDispatcher {
	return &ForecastDispatcher{configs: configs, providers: providers, locations: locations, collector: collector, logger: logger}
}

// Dispatch implements Dispatcher.
func (d *ForecastDispatcher) Dispatch(ctx context.Context, slot *Slot) (int, error) {
	if slot.JobType != JobForecastCollection {
		return 0, fmt.Errorf("unsupported job type %q", slot.JobType)
	}
	if slot.LocationID == nil {
		return 0, fmt.Errorf("forecast slot %s missing location_id", slot.ID)
	}
	cfg, err := d.configs.GetConfiguration(ctx, slot.ProviderConfigurationID)
	if err != nil {
		return 0, fmt.Errorf("load configuration: %w", err)
	}
	provider, err := d.providers.GetProvider(ctx, cfg.ProviderID)
	if err != nil {
		return 0, fmt.Errorf("load provider: %w", err)
	}
	location, err := d.locations.GetLocation(ctx, *slot.LocationID)
	if err != nil {
		return 0, fmt.Errorf("load location: %w", err)
	}

	coll, err := d.collector.Collect(ctx, collection.CollectInput{
		Provider: provider,
		Location: location,
		Config:   cfg,
		Actor:    catalog.Actor{Name: "scheduler"},
		Source:   collection.SourceScheduled,
	})
	if err != nil {
		return 0, err
	}
	return coll.SnapshotsStored, nil
}

// Router dispatches a slot to the per-job-type dispatcher. The scheduler holds a
// single Dispatcher; Router lets one worker serve multiple job types
// (forecast_collection, observation_collection, …) selected by slot.JobType.
type Router struct {
	byType map[string]Dispatcher
}

// NewRouter builds a Router from a job-type → dispatcher map.
func NewRouter(byType map[string]Dispatcher) *Router {
	return &Router{byType: byType}
}

// Dispatch implements Dispatcher, routing on slot.JobType.
func (r *Router) Dispatch(ctx context.Context, slot *Slot) (int, error) {
	d, ok := r.byType[slot.JobType]
	if !ok {
		return 0, fmt.Errorf("no dispatcher for job type %q", slot.JobType)
	}
	return d.Dispatch(ctx, slot)
}
