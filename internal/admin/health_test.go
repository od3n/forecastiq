package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

type fakeHealthRepo struct {
	cells    []CellHealth
	circuits []CircuitHealth
	obs      []ObservationLocation
	lag      *int64
	err      error
}

func (f *fakeHealthRepo) Cells(context.Context, dbtx.DBTX, time.Time) ([]CellHealth, error) {
	return f.cells, f.err
}
func (f *fakeHealthRepo) Circuits(context.Context, dbtx.DBTX) ([]CircuitHealth, error) {
	return f.circuits, nil
}
func (f *fakeHealthRepo) ObservationLocations(context.Context, dbtx.DBTX, time.Time) ([]ObservationLocation, error) {
	return f.obs, nil
}
func (f *fakeHealthRepo) EngineLagSeconds(context.Context, dbtx.DBTX, time.Time) (*int64, error) {
	return f.lag, nil
}

type fakeStater struct {
	u   VolumeUsage
	err error
}

func (f fakeStater) Usage() (VolumeUsage, error) { return f.u, f.err }

type fakeBackup struct {
	lb, lr *BackupStatus
	err    error
}

func (f fakeBackup) Read() (*BackupStatus, *BackupStatus, error) { return f.lb, f.lr, f.err }

func TestHealthService_Assemble(t *testing.T) {
	lag := int64(120)
	repo := &fakeHealthRepo{
		cells:    []CellHealth{{ProviderID: uuid.New(), ProviderName: "Open-Meteo", LocationID: uuid.New(), LocationName: "JB"}},
		circuits: []CircuitHealth{{ProviderID: uuid.New(), State: "closed"}},
		obs:      []ObservationLocation{{LocationID: uuid.New(), LocationName: "JB", SuspectCount24h: 1}},
		lag:      &lag,
	}
	svc := NewHealthService(repo, nil,
		fakeStater{u: VolumeUsage{UsedBytes: 50, TotalBytes: 100, UsedPct: 50}},
		fakeBackup{lb: &BackupStatus{Status: "success"}}, clock.Fixed{T: time.Now()})

	h, err := svc.Assemble(context.Background())
	require.NoError(t, err)
	assert.Len(t, h.Cells, 1)
	assert.Len(t, h.Circuits, 1)
	assert.Len(t, h.ObservationLocations, 1)
	require.NotNil(t, h.EngineLagSeconds)
	assert.EqualValues(t, 120, *h.EngineLagSeconds)
	require.NotNil(t, h.Volume)
	assert.EqualValues(t, 50, h.Volume.UsedPct)
	require.NotNil(t, h.LastBackup)
	assert.Equal(t, "success", h.LastBackup.Status)
}

func TestHealthService_OptionalSectionsDegrade(t *testing.T) {
	// A statfs/backup error omits the section rather than failing the view.
	svc := NewHealthService(&fakeHealthRepo{}, nil,
		fakeStater{err: errors.New("statfs boom")}, fakeBackup{err: errors.New("no file")}, clock.Fixed{T: time.Now()})
	h, err := svc.Assemble(context.Background())
	require.NoError(t, err)
	assert.Nil(t, h.Volume)
	assert.Nil(t, h.LastBackup)
}

func TestHealthService_RepoErrorFails(t *testing.T) {
	svc := NewHealthService(&fakeHealthRepo{err: errors.New("db down")}, nil, nil, nil, clock.Fixed{T: time.Now()})
	_, err := svc.Assemble(context.Background())
	require.Error(t, err)
}
