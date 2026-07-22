// Package events implements the in-process versioned event seam
// (ADR-006 / ADR-021; module architecture §5). Delivery is synchronous
// in-process calls; payload shapes are frozen so a future NATS transport is
// a swap, not a redesign. Events are advisory hints — the batch schedule is
// the authority — so consumers missing an event on rollback is by design
// (ADR-027). Publishing happens after the producing transaction commits.
package events

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Event is the contract every domain event satisfies. SchemaVersion is
// bumped (never silently changed) when a payload shape evolves.
type Event interface {
	Name() string
	SchemaVersion() int
}

// Handler consumes an event.
type Handler func(ctx context.Context, e Event)

// Bus publishes events to in-process subscribers.
type Bus interface {
	Publish(ctx context.Context, e Event)
	Subscribe(name string, h Handler)
}

// ── Frozen event payloads (ADR-021) ───────────────────────────────────

// ForecastCollected is emitted after a forecast collection commits.
type ForecastCollected struct {
	SchemaVersion_ int
	CollectionID   uuid.UUID
	ProviderID     uuid.UUID
	LocationID     uuid.UUID
	SnapshotCount  int
	Status         string
	At             time.Time
}

// Name implements Event.
func (ForecastCollected) Name() string { return "forecast.collected" }

// SchemaVersion implements Event.
func (e ForecastCollected) SchemaVersion() int { return orOne(e.SchemaVersion_) }

// ObservationCollected is emitted after an observation import commits.
// (Wired by the observation-collection work package; shape frozen here.)
type ObservationCollected struct {
	SchemaVersion_     int
	LocationID         uuid.UUID
	Source             string
	Count              int
	ObservationTypeMix map[string]int
}

// Name implements Event.
func (ObservationCollected) Name() string { return "observation.collected" }

// SchemaVersion implements Event.
func (e ObservationCollected) SchemaVersion() int { return orOne(e.SchemaVersion_) }

// ObservationCorrected triggers rematch + recompute of the affected scope.
type ObservationCorrected struct {
	SchemaVersion_          int
	LocationID              uuid.UUID
	ObservedAt              time.Time
	SupersededObservationID uuid.UUID
	NewObservationID        uuid.UUID
}

// Name implements Event.
func (ObservationCorrected) Name() string { return "observation.corrected" }

// SchemaVersion implements Event.
func (e ObservationCorrected) SchemaVersion() int { return orOne(e.SchemaVersion_) }

// AccuracyCalculated is emitted after an analysis batch commits.
type AccuracyCalculated struct {
	SchemaVersion_  int
	BatchID         uuid.UUID
	Scope           string
	MetricsWritten  int
	RankingsWritten int
}

// Name implements Event.
func (AccuracyCalculated) Name() string { return "accuracy.calculated" }

// SchemaVersion implements Event.
func (e AccuracyCalculated) SchemaVersion() int { return orOne(e.SchemaVersion_) }

// ProviderHealthChanged is emitted on circuit-state transitions.
type ProviderHealthChanged struct {
	SchemaVersion_      int
	ProviderID          uuid.UUID
	OldState            string
	NewState            string
	ConsecutiveFailures int
}

// Name implements Event.
func (ProviderHealthChanged) Name() string { return "provider.health_changed" }

// SchemaVersion implements Event.
func (e ProviderHealthChanged) SchemaVersion() int { return orOne(e.SchemaVersion_) }

func orOne(v int) int {
	if v == 0 {
		return 1
	}
	return v
}

// ── Bus implementation ────────────────────────────────────────────────

// SyncBus is a synchronous in-process Bus. Handlers run in the publisher's
// goroutine; a panicking handler is recovered and logged so one bad
// subscriber cannot break the producer.
type SyncBus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
	logger   *slog.Logger
}

// NewSyncBus returns an empty synchronous bus.
func NewSyncBus(logger *slog.Logger) *SyncBus {
	return &SyncBus{handlers: make(map[string][]Handler), logger: logger}
}

// Subscribe registers a handler for an event name.
func (b *SyncBus) Subscribe(name string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[name] = append(b.handlers[name], h)
}

// Publish dispatches the event to all subscribers of its name.
func (b *SyncBus) Publish(ctx context.Context, e Event) {
	b.mu.RLock()
	handlers := append([]Handler(nil), b.handlers[e.Name()]...)
	b.mu.RUnlock()

	for _, h := range handlers {
		func() {
			defer func() {
				if p := recover(); p != nil && b.logger != nil {
					b.logger.Error("event.handler_panicked",
						slog.String("event", e.Name()),
						slog.Any("panic", p))
				}
			}()
			h(ctx, e)
		}()
	}
}
