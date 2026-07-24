// Package admin is the operations module's application layer (WP-18): it
// assembles the S-10 collection-health view from application tables, filesystem
// stats, and the backup status file — never from logs or the metrics system
// (operations doc §4 binding rule). It owns no writes; admin mutations live in
// their owning modules (catalog, identity) and are invoked by the API layer.
package admin

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// CellHealth is the collection health of one provider×location cell (S-10):
// last successful collection, the most recent terminal status, and the next
// scheduled slot. Freshness state is computed by the API layer from LastSuccess.
type CellHealth struct {
	ProviderID      uuid.UUID
	ProviderName    string
	ProviderSlug    string
	LocationID      uuid.UUID
	LocationName    string
	LastSuccessAt   *time.Time
	LastStatus      string
	NextScheduledAt *time.Time
}

// CircuitHealth is one provider's breaker state (S-10; provider_circuits).
type CircuitHealth struct {
	ProviderID          uuid.UUID
	ProviderName        string
	State               string
	ConsecutiveFailures int
	NextProbeAt         *time.Time
}

// ObservationLocation is per-location observation-collector health (S-10).
type ObservationLocation struct {
	LocationID      uuid.UUID
	LocationName    string
	LastObservedAt  *time.Time
	SuspectCount24h int
}

// VolumeUsage is the payload volume's block usage (statfs; S-10 system section).
type VolumeUsage struct {
	UsedBytes  uint64
	TotalBytes uint64
	UsedPct    float64
}

// BackupStatus is one backup/restore-test status entry (from the status file).
type BackupStatus struct {
	CompletedAt *time.Time
	Status      string
}

// Health is the assembled S-10 payload (pre-DTO). EngineLagSeconds is
// now − max(accuracy_metrics.calculated_at); nil when no metrics exist.
type Health struct {
	Cells                []CellHealth
	Circuits             []CircuitHealth
	ObservationLocations []ObservationLocation
	Volume               *VolumeUsage
	EngineLagSeconds     *int64
	LastBackup           *BackupStatus
	LastRestoreTest      *BackupStatus
	GeneratedAt          time.Time
}

// HealthRepository reads the DB-backed health aggregates (adminpg). All queries
// hit application tables only (operations doc §4). now anchors the "future
// slot" and 24 h windows.
type HealthRepository interface {
	Cells(ctx context.Context, tx dbtx.DBTX, now time.Time) ([]CellHealth, error)
	Circuits(ctx context.Context, tx dbtx.DBTX) ([]CircuitHealth, error)
	ObservationLocations(ctx context.Context, tx dbtx.DBTX, now time.Time) ([]ObservationLocation, error)
	EngineLagSeconds(ctx context.Context, tx dbtx.DBTX, now time.Time) (*int64, error)
}

// VolumeStater reports payload-volume usage (statfs; implemented over the
// payload store in the composition root).
type VolumeStater interface {
	Usage() (VolumeUsage, error)
}

// BackupStatusReader reads the backup/restore-test status file (last_backup,
// last_restore_test). Both are nil when the file is absent or unset.
type BackupStatusReader interface {
	Read() (lastBackup, lastRestoreTest *BackupStatus, err error)
}

// HealthService assembles the S-10 health view. Reads use the pool directly.
type HealthService struct {
	repo   HealthRepository
	pool   dbtx.DBTX
	volume VolumeStater
	backup BackupStatusReader
	clock  clock.Clock
}

// NewHealthService wires a HealthService. volume and backup may be nil (their
// sections are then omitted).
func NewHealthService(repo HealthRepository, pool dbtx.DBTX, volume VolumeStater, backup BackupStatusReader, clk clock.Clock) *HealthService {
	return &HealthService{repo: repo, pool: pool, volume: volume, backup: backup, clock: clk}
}

// Assemble builds the full health view. Section failures are surfaced as errors
// (the admin screen prefers an explicit failure to a silently partial view),
// except the optional volume/backup sections which degrade to omitted on error.
func (s *HealthService) Assemble(ctx context.Context) (*Health, error) {
	now := s.clock.Now().UTC()
	h := &Health{GeneratedAt: now}

	cells, err := s.repo.Cells(ctx, s.pool, now)
	if err != nil {
		return nil, err
	}
	h.Cells = cells

	circuits, err := s.repo.Circuits(ctx, s.pool)
	if err != nil {
		return nil, err
	}
	h.Circuits = circuits

	obs, err := s.repo.ObservationLocations(ctx, s.pool, now)
	if err != nil {
		return nil, err
	}
	h.ObservationLocations = obs

	lag, err := s.repo.EngineLagSeconds(ctx, s.pool, now)
	if err != nil {
		return nil, err
	}
	h.EngineLagSeconds = lag

	if s.volume != nil {
		if u, verr := s.volume.Usage(); verr == nil {
			h.Volume = &u
		}
	}
	if s.backup != nil {
		if lb, lr, berr := s.backup.Read(); berr == nil {
			h.LastBackup, h.LastRestoreTest = lb, lr
		}
	}
	return h, nil
}
