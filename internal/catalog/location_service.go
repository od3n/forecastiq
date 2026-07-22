package catalog

import (
	"context"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/audit"
	"github.com/forecastiq/forecastiq/internal/catalog/domain"
	"github.com/forecastiq/forecastiq/internal/catalog/ports"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
	"github.com/forecastiq/forecastiq/internal/platform/ids"
)

// LocationService implements LocationManager.
type LocationService struct {
	repo   ports.LocationRepository
	tx     *dbtx.Runner
	pool   dbtx.DBTX
	audit  audit.Recorder
	clock  clock.Clock
	logger *slog.Logger
}

// NewLocationService wires a LocationService.
func NewLocationService(repo ports.LocationRepository, tx *dbtx.Runner, pool dbtx.DBTX,
	rec audit.Recorder, clk clock.Clock, logger *slog.Logger) *LocationService {
	return &LocationService{repo: repo, tx: tx, pool: pool, audit: rec, clock: clk, logger: logger}
}

// CreateLocation validates, applies the BR-LOC-01 dedup check, persists, and
// audits — all in one transaction.
func (s *LocationService) CreateLocation(ctx context.Context, in CreateLocationInput) (*domain.Location, error) {
	now := s.clock.Now()
	workspaceID := in.WorkspaceID
	if workspaceID == uuid.Nil {
		workspaceID = domain.SystemWorkspaceID // MVP: single system workspace
	}
	loc := &domain.Location{
		ID:          ids.New(),
		WorkspaceID: workspaceID,
		Name:        in.Name,
		Latitude:    in.Latitude,
		Longitude:   in.Longitude,
		CountryCode: in.CountryCode,
		Timezone:    in.Timezone,
		Status:      domain.StatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := loc.ValidateCreation(); err != nil {
		return nil, err
	}
	// DRB-WP04-003: override reason is mandatory when the dedup bypass is used,
	// so every near-duplicate acceptance in the audit trail is accountable.
	if in.AllowNearDuplicate && strings.TrimSpace(in.OverrideReason) == "" {
		return nil, &domain.ValidationError{Fields: []domain.FieldError{
			{Field: "override_reason", Message: "required when allow_near_duplicate is true"},
		}}
	}

	err := s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		// Serialize the dedup window: the advisory lock prevents concurrent
		// creates from both passing the proximity check before either commits
		// (DRB-WP04-001). Released automatically on commit/rollback.
		if err := s.repo.AcquireDedupLock(ctx, tx); err != nil {
			return err
		}
		active, err := s.repo.ListActive(ctx, tx)
		if err != nil {
			return err
		}
		if !in.AllowNearDuplicate {
			for _, existing := range active {
				dist := domain.HaversineDegrees(loc.Latitude, loc.Longitude, existing.Latitude, existing.Longitude)
				if domain.IsNearDuplicate(dist) {
					return &domain.DuplicateLocationError{
						ExistingID:      existing.ID,
						ExistingName:    existing.Name,
						DistanceDegrees: dist,
					}
				}
			}
		}
		if err := s.repo.Insert(ctx, tx, loc); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Event{
			UserID:       in.Actor.UserID,
			Action:       "location.create",
			ResourceType: "location",
			ResourceID:   &loc.ID,
			IPAddress:    in.Actor.IPAddress,
			Details: map[string]any{
				"actor":                actorName(in.Actor),
				"name":                 loc.Name,
				"allow_near_duplicate": in.AllowNearDuplicate,
				"override_reason":      in.OverrideReason,
			},
		})
	})
	if err != nil {
		return nil, err
	}
	s.logger.InfoContext(ctx, "location.created",
		slog.String("location_id", loc.ID.String()),
		slog.String("name", loc.Name))
	return loc, nil
}

// GetLocation returns a location by id (404 when unknown).
func (s *LocationService) GetLocation(ctx context.Context, id uuid.UUID) (*domain.Location, error) {
	return s.repo.GetByID(ctx, s.pool, id)
}

// ListActiveLocations returns all active locations (used by the scheduler to
// generate collection slots).
func (s *LocationService) ListActiveLocations(ctx context.Context) ([]*domain.Location, error) {
	return s.repo.ListActive(ctx, s.pool)
}

// ListLocations returns a keyset-paginated location page.
func (s *LocationService) ListLocations(ctx context.Context, in ListLocationsInput) ([]*domain.Location, PageInfo, error) {
	limit := in.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var cursor uuid.UUID
	if in.Cursor != "" {
		parsed, err := ids.Parse(in.Cursor)
		if err != nil {
			return nil, PageInfo{}, &domain.ValidationError{Fields: []domain.FieldError{
				{Field: "cursor", Message: "must be a valid UUID"},
			}}
		}
		cursor = parsed
	}
	rows, err := s.repo.List(ctx, s.pool, ports.LocationFilter{Active: in.Active, Cursor: cursor, Limit: limit})
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

// UpdateLocation updates mutable fields (name only — timezone and coordinates
// are immutable per domain architecture §2.3) and audits.
func (s *LocationService) UpdateLocation(ctx context.Context, id uuid.UUID, in UpdateLocationInput) (*domain.Location, error) {
	var updated *domain.Location
	err := s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		loc, err := s.repo.GetByID(ctx, tx, id)
		if err != nil {
			return err
		}
		changes := map[string]any{}
		if in.Name != nil {
			loc.Name = *in.Name
			changes["name"] = *in.Name
		}
		if err := loc.ValidateCreation(); err != nil {
			return err
		}
		if err := s.repo.Update(ctx, tx, loc); err != nil {
			return err
		}
		updated = loc
		return s.audit.Record(ctx, tx, audit.Event{
			UserID:       in.Actor.UserID,
			Action:       "location.update",
			ResourceType: "location",
			ResourceID:   &loc.ID,
			IPAddress:    in.Actor.IPAddress,
			Details:      map[string]any{"actor": actorName(in.Actor), "changes": changes},
		})
	})
	if err != nil {
		return nil, err
	}
	s.logger.InfoContext(ctx, "location.updated",
		slog.String("location_id", updated.ID.String()),
		slog.String("name", updated.Name))
	return updated, nil
}

// SetLocationStatus enables/disables a location and audits. Disabling stops
// future collection; historical data remains (BR-LOC-03). Only active ↔
// disabled transitions are permitted; archived is reserved (DRB-WP04-004).
func (s *LocationService) SetLocationStatus(ctx context.Context, id uuid.UUID, status domain.Status, actor Actor) (*domain.Location, error) {
	if !status.Settable() {
		return nil, &domain.ValidationError{Fields: []domain.FieldError{
			{Field: "status", Message: "must be one of active|disabled"},
		}}
	}
	var updated *domain.Location
	err := s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		loc, err := s.repo.GetByID(ctx, tx, id)
		if err != nil {
			return err
		}
		// Reject no-op transitions (DRB-WP04-004): they pollute the audit trail.
		if loc.Status == status {
			return &domain.StatusTransitionError{From: loc.Status, To: status}
		}
		if err := s.repo.UpdateStatus(ctx, tx, id, status); err != nil {
			return err
		}
		loc.Status = status
		updated = loc
		return s.audit.Record(ctx, tx, audit.Event{
			UserID:       actor.UserID,
			Action:       "location.set_status",
			ResourceType: "location",
			ResourceID:   &id,
			IPAddress:    actor.IPAddress,
			Details:      map[string]any{"actor": actorName(actor), "status": string(status)},
		})
	})
	if err != nil {
		return nil, err
	}
	s.logger.InfoContext(ctx, "location.status_changed",
		slog.String("location_id", id.String()),
		slog.String("status", string(status)))
	return updated, nil
}

func actorName(a Actor) string {
	if a.Name != "" {
		return a.Name
	}
	if a.UserID != nil {
		return a.UserID.String()
	}
	return "system"
}
