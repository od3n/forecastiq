package catalog_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/internal/audit"
	"github.com/forecastiq/forecastiq/internal/catalog"
	"github.com/forecastiq/forecastiq/internal/catalog/domain"
	"github.com/forecastiq/forecastiq/internal/catalog/ports"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// ── Fakes ────────────────────────────────────────────────────────────

// fakeTx satisfies pgx.Tx for the Runner; only Commit/Rollback are exercised
// because the fake repositories ignore the tx handle.
type fakeTx struct{ pgx.Tx }

func (fakeTx) Commit(context.Context) error   { return nil }
func (fakeTx) Rollback(context.Context) error { return nil }

// fakePool satisfies dbtx.TxBeginner and dbtx.DBTX (query methods are never
// exercised because the fake repositories ignore the tx handle).
type fakePool struct{}

func (fakePool) Begin(context.Context) (pgx.Tx, error) { return fakeTx{}, nil }

func (fakePool) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (fakePool) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }
func (fakePool) QueryRow(context.Context, string, ...any) pgx.Row        { return nil }

// fakeLocationRepo is an in-memory ports.LocationRepository.
type fakeLocationRepo struct {
	rows map[uuid.UUID]*domain.Location
}

func newFakeLocationRepo() *fakeLocationRepo {
	return &fakeLocationRepo{rows: map[uuid.UUID]*domain.Location{}}
}

func (r *fakeLocationRepo) Insert(_ context.Context, _ dbtx.DBTX, l *domain.Location) error {
	cp := *l
	r.rows[l.ID] = &cp
	return nil
}

func (r *fakeLocationRepo) GetByID(_ context.Context, _ dbtx.DBTX, id uuid.UUID) (*domain.Location, error) {
	l, ok := r.rows[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *l
	return &cp, nil
}

func (r *fakeLocationRepo) List(_ context.Context, _ dbtx.DBTX, f ports.LocationFilter) ([]*domain.Location, error) {
	var out []*domain.Location
	for _, l := range r.rows {
		if l.ID.String() <= f.Cursor.String() {
			continue
		}
		if f.Active != nil && l.Status.Active() != *f.Active {
			continue
		}
		out = append(out, l)
	}
	// Simple insertion sort by ID for deterministic keyset order.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].ID.String() < out[j-1].ID.String(); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	if len(out) > f.Limit+1 {
		out = out[:f.Limit+1]
	}
	return out, nil
}

func (r *fakeLocationRepo) ListActive(ctx context.Context, tx dbtx.DBTX) ([]*domain.Location, error) {
	active := true
	return r.List(ctx, tx, ports.LocationFilter{Active: &active, Limit: len(r.rows) + 1})
}

func (r *fakeLocationRepo) Update(_ context.Context, _ dbtx.DBTX, l *domain.Location) error {
	if _, ok := r.rows[l.ID]; !ok {
		return domain.ErrNotFound
	}
	cp := *l
	r.rows[l.ID] = &cp
	return nil
}

func (r *fakeLocationRepo) UpdateStatus(_ context.Context, _ dbtx.DBTX, id uuid.UUID, status domain.Status) error {
	l, ok := r.rows[id]
	if !ok {
		return domain.ErrNotFound
	}
	l.Status = status
	return nil
}

// fakeAudit captures recorded events.
type fakeAudit struct {
	events []audit.Event
}

func (f *fakeAudit) Record(_ context.Context, _ dbtx.DBTX, e audit.Event) error {
	f.events = append(f.events, e)
	return nil
}

// newService wires a LocationService over the fakes.
func newService(repo ports.LocationRepository, rec audit.Recorder) *catalog.LocationService {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return catalog.NewLocationService(repo, dbtx.NewRunner(fakePool{}), fakePool{}, rec,
		clock.Fixed{T: time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)}, logger)
}

func jbInput() catalog.CreateLocationInput {
	return catalog.CreateLocationInput{
		Name: "Johor Bahru", Latitude: 1.4927, Longitude: 103.7414,
		CountryCode: "MY", Timezone: "Asia/Kuala_Lumpur",
	}
}

// ── Create ───────────────────────────────────────────────────────────

func TestCreateLocation_Success(t *testing.T) {
	repo := newFakeLocationRepo()
	rec := &fakeAudit{}
	svc := newService(repo, rec)

	loc, err := svc.CreateLocation(context.Background(), jbInput())
	require.NoError(t, err)
	assert.Equal(t, domain.StatusActive, loc.Status)
	assert.Equal(t, domain.SystemWorkspaceID, loc.WorkspaceID)
	assert.NotEqual(t, uuid.Nil, loc.ID)
	assert.Len(t, repo.rows, 1)

	require.Len(t, rec.events, 1)
	assert.Equal(t, "location.create", rec.events[0].Action)
	assert.Equal(t, "location", rec.events[0].ResourceType)
	assert.Equal(t, loc.ID, *rec.events[0].ResourceID)
}

func TestCreateLocation_Validation(t *testing.T) {
	svc := newService(newFakeLocationRepo(), &fakeAudit{})
	in := jbInput()
	in.Latitude = 91
	in.CountryCode = "my"

	_, err := svc.CreateLocation(context.Background(), in)
	require.Error(t, err)
	var ve *domain.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.GreaterOrEqual(t, len(ve.Fields), 2)
}

func TestCreateLocation_DuplicateRejected(t *testing.T) {
	repo := newFakeLocationRepo()
	rec := &fakeAudit{}
	svc := newService(repo, rec)

	first, err := svc.CreateLocation(context.Background(), jbInput())
	require.NoError(t, err)

	// 0.01° north of the existing location — inside the BR-LOC-01 boundary.
	near := jbInput()
	near.Name = "JB Copy"
	near.Latitude = 1.5027

	_, err = svc.CreateLocation(context.Background(), near)
	require.Error(t, err)
	var dupErr *domain.DuplicateLocationError
	require.ErrorAs(t, err, &dupErr)
	assert.Equal(t, first.ID, dupErr.ExistingID)
	assert.Equal(t, "Johor Bahru", dupErr.ExistingName)
	assert.Less(t, dupErr.DistanceDegrees, domain.DedupThresholdDegrees)
	assert.Len(t, rec.events, 1, "rejected create must not audit")
}

func TestCreateLocation_DuplicateBoundaryPermitted(t *testing.T) {
	repo := newFakeLocationRepo()
	svc := newService(repo, &fakeAudit{})

	_, err := svc.CreateLocation(context.Background(), jbInput())
	require.NoError(t, err)

	// Exactly 0.05° of latitude away — permitted (strictly-less-than rule).
	boundary := jbInput()
	boundary.Name = "Boundary Point"
	boundary.Latitude = 1.4927 + domain.DedupThresholdDegrees

	loc, err := svc.CreateLocation(context.Background(), boundary)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, loc.ID)
	assert.Len(t, repo.rows, 2)
}

func TestCreateLocation_OverrideWithReason(t *testing.T) {
	repo := newFakeLocationRepo()
	rec := &fakeAudit{}
	svc := newService(repo, rec)

	_, err := svc.CreateLocation(context.Background(), jbInput())
	require.NoError(t, err)

	near := jbInput()
	near.Name = "JB Harbour"
	near.Latitude = 1.5027
	near.AllowNearDuplicate = true
	near.OverrideReason = "distinct harbour district monitoring site"

	loc, err := svc.CreateLocation(context.Background(), near)
	require.NoError(t, err)
	assert.Len(t, repo.rows, 2)

	require.Len(t, rec.events, 2)
	details := rec.events[1].Details
	assert.Equal(t, true, details["allow_near_duplicate"])
	assert.Equal(t, "distinct harbour district monitoring site", details["override_reason"])
	assert.Equal(t, loc.ID, *rec.events[1].ResourceID)
}

func TestCreateLocation_DisabledLocationsNotDeduped(t *testing.T) {
	repo := newFakeLocationRepo()
	svc := newService(repo, &fakeAudit{})

	first, err := svc.CreateLocation(context.Background(), jbInput())
	require.NoError(t, err)
	require.NoError(t, repo.UpdateStatus(context.Background(), nil, first.ID, domain.StatusDisabled))

	// Same coordinates as a disabled location — permitted (dedup applies to
	// active locations only).
	near := jbInput()
	near.Name = "JB Replacement"
	loc, err := svc.CreateLocation(context.Background(), near)
	require.NoError(t, err)
	assert.NotEqual(t, first.ID, loc.ID)
}

// ── Update ───────────────────────────────────────────────────────────

func TestUpdateLocation_NameOnly(t *testing.T) {
	repo := newFakeLocationRepo()
	rec := &fakeAudit{}
	svc := newService(repo, rec)

	created, err := svc.CreateLocation(context.Background(), jbInput())
	require.NoError(t, err)

	newName := "Johor Bahru City"
	updated, err := svc.UpdateLocation(context.Background(), created.ID, catalog.UpdateLocationInput{
		Name: &newName,
	})
	require.NoError(t, err)
	assert.Equal(t, newName, updated.Name)
	// Immutable fields unchanged.
	assert.Equal(t, created.Timezone, updated.Timezone)
	assert.Equal(t, created.Latitude, updated.Latitude)
	assert.Equal(t, created.Longitude, updated.Longitude)

	require.Len(t, rec.events, 2)
	assert.Equal(t, "location.update", rec.events[1].Action)
	changes := rec.events[1].Details["changes"].(map[string]any)
	assert.Equal(t, newName, changes["name"])
	_, hasTZ := changes["timezone"]
	assert.False(t, hasTZ, "timezone must not be mutable")
}

func TestUpdateLocation_InvalidName(t *testing.T) {
	repo := newFakeLocationRepo()
	svc := newService(repo, &fakeAudit{})

	created, err := svc.CreateLocation(context.Background(), jbInput())
	require.NoError(t, err)

	empty := ""
	_, err = svc.UpdateLocation(context.Background(), created.ID, catalog.UpdateLocationInput{Name: &empty})
	require.Error(t, err)
	var ve *domain.ValidationError
	assert.ErrorAs(t, err, &ve)
}

func TestUpdateLocation_NotFound(t *testing.T) {
	svc := newService(newFakeLocationRepo(), &fakeAudit{})
	name := "X"
	_, err := svc.UpdateLocation(context.Background(), uuid.Must(uuid.NewV7()), catalog.UpdateLocationInput{Name: &name})
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// ── Status lifecycle ─────────────────────────────────────────────────

func TestSetLocationStatus_Disable(t *testing.T) {
	repo := newFakeLocationRepo()
	rec := &fakeAudit{}
	svc := newService(repo, rec)

	created, err := svc.CreateLocation(context.Background(), jbInput())
	require.NoError(t, err)

	updated, err := svc.SetLocationStatus(context.Background(), created.ID, domain.StatusDisabled, catalog.Actor{Name: "op"})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusDisabled, updated.Status)

	require.Len(t, rec.events, 2)
	assert.Equal(t, "location.set_status", rec.events[1].Action)
	assert.Equal(t, "disabled", rec.events[1].Details["status"])
}

func TestSetLocationStatus_Invalid(t *testing.T) {
	svc := newService(newFakeLocationRepo(), &fakeAudit{})
	_, err := svc.SetLocationStatus(context.Background(), uuid.Must(uuid.NewV7()), "bogus", catalog.Actor{})
	require.Error(t, err)
	var ve *domain.ValidationError
	assert.ErrorAs(t, err, &ve)
}

func TestSetLocationStatus_NotFound(t *testing.T) {
	svc := newService(newFakeLocationRepo(), &fakeAudit{})
	_, err := svc.SetLocationStatus(context.Background(), uuid.Must(uuid.NewV7()), domain.StatusDisabled, catalog.Actor{})
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// ── Collection eligibility (BR-LOC-03) ──────────────────────────────

func TestListActiveLocations_ExcludesDisabled(t *testing.T) {
	repo := newFakeLocationRepo()
	svc := newService(repo, &fakeAudit{})

	created, err := svc.CreateLocation(context.Background(), jbInput())
	require.NoError(t, err)

	active, err := svc.ListActiveLocations(context.Background())
	require.NoError(t, err)
	assert.Len(t, active, 1)

	_, err = svc.SetLocationStatus(context.Background(), created.ID, domain.StatusDisabled, catalog.Actor{})
	require.NoError(t, err)

	active, err = svc.ListActiveLocations(context.Background())
	require.NoError(t, err)
	assert.Empty(t, active, "disabled locations must not be collection-eligible")
}

// ── Listing / pagination ─────────────────────────────────────────────

func TestListLocations_Pagination(t *testing.T) {
	repo := newFakeLocationRepo()
	svc := newService(repo, &fakeAudit{})

	for i := range 3 {
		in := jbInput()
		in.Name = "Loc " + string(rune('A'+i))
		in.Latitude = float64(10 + i) // far apart — no dedup
		_, err := svc.CreateLocation(context.Background(), in)
		require.NoError(t, err)
	}

	page1, info, err := svc.ListLocations(context.Background(), catalog.ListLocationsInput{Limit: 2})
	require.NoError(t, err)
	assert.Len(t, page1, 2)
	assert.True(t, info.HasMore)
	assert.NotEmpty(t, info.NextCursor)

	page2, info2, err := svc.ListLocations(context.Background(), catalog.ListLocationsInput{Limit: 2, Cursor: info.NextCursor})
	require.NoError(t, err)
	assert.Len(t, page2, 1)
	assert.False(t, info2.HasMore)
}

func TestListLocations_InvalidCursor(t *testing.T) {
	svc := newService(newFakeLocationRepo(), &fakeAudit{})
	_, _, err := svc.ListLocations(context.Background(), catalog.ListLocationsInput{Cursor: "not-a-uuid"})
	require.Error(t, err)
	var ve *domain.ValidationError
	assert.ErrorAs(t, err, &ve)
}
