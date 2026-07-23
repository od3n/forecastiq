package audit

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// RecordedEvent is a stored audit row (read model). It includes the row id and
// timestamp that the write-side Event omits.
type RecordedEvent struct {
	ID           uuid.UUID
	UserID       *uuid.UUID
	Action       string
	ResourceType string
	ResourceID   *uuid.UUID
	Details      map[string]any
	IPAddress    string
	CreatedAt    time.Time
}

// Filter selects audit events for reading. All predicates are optional; Cursor
// is the last-seen id (exclusive) for keyset pagination over the time-ordered
// (UUIDv7) id, newest first.
type Filter struct {
	Action       *string
	ResourceType *string
	UserID       *uuid.UUID
	Cursor       uuid.UUID
	Limit        int
}

// PageInfo is a keyset-pagination result.
type PageInfo struct {
	HasMore    bool
	NextCursor string
}

// QueryStore reads audit events. Implemented by adapters/persistence/auditpg.
// Reads use the pool directly (no transaction). It returns up to filter.Limit+1
// rows so the reader can detect has_more.
type QueryStore interface {
	List(ctx context.Context, pool dbtx.DBTX, f Filter) ([]RecordedEvent, error)
}

// Reader is the audit read model (powers the admin audit screen in WP-18).
type Reader interface {
	List(ctx context.Context, f Filter) ([]RecordedEvent, PageInfo, error)
}

// ReaderService implements Reader over a QueryStore.
type ReaderService struct {
	store QueryStore
	pool  dbtx.DBTX
}

// NewReaderService wires a ReaderService.
func NewReaderService(store QueryStore, pool dbtx.DBTX) *ReaderService {
	return &ReaderService{store: store, pool: pool}
}

// List returns a keyset-paginated page of audit events (newest first).
func (s *ReaderService) List(ctx context.Context, f Filter) ([]RecordedEvent, PageInfo, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	rows, err := s.store.List(ctx, s.pool, f)
	if err != nil {
		return nil, PageInfo{}, err
	}
	page := PageInfo{}
	if len(rows) > f.Limit {
		rows = rows[:f.Limit]
		page.HasMore = true
		page.NextCursor = rows[len(rows)-1].ID.String()
	}
	return rows, page, nil
}
