// Package audit implements the append-only audit seam (module architecture
// §3.8; security architecture §4). Audit recording is mandatory for admin
// mutations and participates in the caller's transaction (same DB, same
// process — no outbox needed, ADR-027). A write failure fails the command.
package audit

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// Event is one audit record. UserID is nil for system/dev-token actors until
// the identity work package (WP-03) lands; the actor is then described in
// Details.
type Event struct {
	UserID       *uuid.UUID
	Action       string // registry: location.create, collection.trigger, ...
	ResourceType string // location, provider, collection, ...
	ResourceID   *uuid.UUID
	Details      map[string]any // sanitized; never credentials or payloads
	IPAddress    string
	At           time.Time
}

// Store persists audit events. Implemented by adapters/persistence/auditpg.
// The dbtx parameter lets the record join the caller's transaction.
type Store interface {
	Insert(ctx context.Context, tx dbtx.DBTX, e Event) error
}

// Recorder records audit events within a transaction.
type Recorder interface {
	Record(ctx context.Context, tx dbtx.DBTX, e Event) error
}

// recorder is the default Recorder backed by a Store.
type recorder struct{ store Store }

// NewRecorder returns a Recorder that writes through the given store.
func NewRecorder(store Store) Recorder { return &recorder{store: store} }

// Record implements Recorder.
func (r *recorder) Record(ctx context.Context, tx dbtx.DBTX, e Event) error {
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	return r.store.Insert(ctx, tx, e)
}

// Nop returns a Recorder that discards events (test helper only).
func Nop() Recorder { return nopRecorder{} }

type nopRecorder struct{}

func (nopRecorder) Record(context.Context, dbtx.DBTX, Event) error { return nil }
