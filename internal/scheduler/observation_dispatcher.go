package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/forecastiq/forecastiq/internal/catalog"
)

// observationWindow is the backfill window each observation collection covers
// (workflow §3: last 2 h, dedup'd by the live-row boundary).
const observationWindow = 2 * time.Hour

// ObservationCollector runs one observation collection for a location-window.
// Implemented by collection.ObserveService.
type ObservationCollector interface {
	Observe(ctx context.Context, location *catalog.Location, windowStart, windowEnd time.Time) (int, error)
}

// ObservationDispatcher dispatches observation_collection slots: it resolves the
// slot's location and collects the 2 h backfill window ending at the slot's
// (hour-aligned) time.
type ObservationDispatcher struct {
	collector ObservationCollector
	locations catalog.LocationManager
	logger    *slog.Logger
}

// NewObservationDispatcher wires an ObservationDispatcher.
func NewObservationDispatcher(collector ObservationCollector, locations catalog.LocationManager, logger *slog.Logger) *ObservationDispatcher {
	return &ObservationDispatcher{collector: collector, locations: locations, logger: logger}
}

// Dispatch implements Dispatcher.
func (d *ObservationDispatcher) Dispatch(ctx context.Context, slot *Slot) (int, error) {
	if slot.JobType != JobObservationCollection {
		return 0, fmt.Errorf("unsupported job type %q", slot.JobType)
	}
	if slot.LocationID == nil {
		return 0, fmt.Errorf("observation slot %s missing location_id", slot.ID)
	}
	location, err := d.locations.GetLocation(ctx, *slot.LocationID)
	if err != nil {
		return 0, fmt.Errorf("load location: %w", err)
	}
	// Hour-align the window so the source receives top-of-hour bounds (slots
	// fire at :05 to allow publication delay; the window covers the last 2 h).
	end := slot.SlotTime.UTC().Truncate(time.Hour)
	start := end.Add(-observationWindow)
	return d.collector.Observe(ctx, location, start, end)
}
