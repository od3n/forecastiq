package scheduler_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/internal/catalog"
	catalogdomain "github.com/forecastiq/forecastiq/internal/catalog/domain"
	"github.com/forecastiq/forecastiq/internal/scheduler"
)

// testLogger returns a discard logger for dispatcher tests.
func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// fakeDispatcher records that it was invoked and returns a fixed count.
type fakeDispatcher struct {
	count  int
	called int
}

func (d *fakeDispatcher) Dispatch(_ context.Context, _ *scheduler.Slot) (int, error) {
	d.called++
	return d.count, nil
}

func TestRouter_RoutesByJobType(t *testing.T) {
	fc := &fakeDispatcher{count: 3}
	oc := &fakeDispatcher{count: 5}
	r := scheduler.NewRouter(map[string]scheduler.Dispatcher{
		scheduler.JobForecastCollection:    fc,
		scheduler.JobObservationCollection: oc,
	})

	n, err := r.Dispatch(context.Background(), &scheduler.Slot{JobType: scheduler.JobObservationCollection})
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, 1, oc.called)
	assert.Equal(t, 0, fc.called, "forecast dispatcher not invoked for an observation slot")

	_, err = r.Dispatch(context.Background(), &scheduler.Slot{JobType: "analysis_batch"})
	require.Error(t, err, "unknown job type is an error, not a silent no-op")
}

// fakeCollector captures the window the dispatcher computed.
type fakeCollector struct {
	loc        *catalog.Location
	start, end time.Time
	n          int
}

func (c *fakeCollector) Observe(_ context.Context, loc *catalog.Location, start, end time.Time) (int, error) {
	c.loc, c.start, c.end, c.n = loc, start, end, 2
	return c.n, nil
}

// fakeLocations is a minimal catalog.LocationManager returning one location.
type fakeLocations struct{ loc *catalog.Location }

func (f *fakeLocations) GetLocation(_ context.Context, _ uuid.UUID) (*catalogdomain.Location, error) {
	return f.loc, nil
}
func (f *fakeLocations) CreateLocation(context.Context, catalog.CreateLocationInput) (*catalogdomain.Location, error) {
	return nil, nil
}
func (f *fakeLocations) ListLocations(context.Context, catalog.ListLocationsInput) ([]*catalogdomain.Location, catalog.PageInfo, error) {
	return nil, catalog.PageInfo{}, nil
}
func (f *fakeLocations) ListActiveLocations(context.Context) ([]*catalogdomain.Location, error) {
	return []*catalogdomain.Location{f.loc}, nil
}
func (f *fakeLocations) UpdateLocation(context.Context, uuid.UUID, catalog.UpdateLocationInput) (*catalogdomain.Location, error) {
	return nil, nil
}
func (f *fakeLocations) SetLocationStatus(context.Context, uuid.UUID, catalogdomain.Status, catalog.Actor) (*catalogdomain.Location, error) {
	return nil, nil
}

// TestObservationDispatcher_Window proves the dispatcher hour-aligns the slot
// time and requests the trailing 2 h window for the slot's location.
func TestObservationDispatcher_Window(t *testing.T) {
	locID := uuid.MustParse("00000000-0000-0000-0000-000000000030")
	loc := &catalog.Location{ID: locID, Latitude: 1.49, Longitude: 103.74, Timezone: "UTC", Status: catalog.StatusActive}
	col := &fakeCollector{}
	d := scheduler.NewObservationDispatcher(col, &fakeLocations{loc: loc}, testLogger())

	slotTime := time.Date(2026, 7, 22, 11, 5, 0, 0, time.UTC) // :05 slot
	n, err := d.Dispatch(context.Background(), &scheduler.Slot{
		JobType: scheduler.JobObservationCollection, LocationID: &locID, SlotTime: slotTime,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Same(t, loc, col.loc)
	// Hour-aligned end (11:00) and a 2 h trailing window (09:00).
	assert.True(t, col.end.Equal(time.Date(2026, 7, 22, 11, 0, 0, 0, time.UTC)), "end hour-aligned")
	assert.True(t, col.start.Equal(time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)), "2h trailing window")
}

func TestObservationDispatcher_RejectsWrongJobType(t *testing.T) {
	d := scheduler.NewObservationDispatcher(&fakeCollector{}, &fakeLocations{}, testLogger())
	_, err := d.Dispatch(context.Background(), &scheduler.Slot{JobType: scheduler.JobForecastCollection})
	require.Error(t, err)
}
