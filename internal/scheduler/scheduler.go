package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/catalog"
	"github.com/forecastiq/forecastiq/internal/collection/domain"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
	"github.com/forecastiq/forecastiq/internal/platform/ids"
	"github.com/forecastiq/forecastiq/internal/platform/metrics"
)

// Config configures a Scheduler.
type Config struct {
	InstanceID    string
	Interval      time.Duration
	LeaseDuration time.Duration
	MaxConcurrent int
	ClaimBatch    int
}

// Scheduler generates due slots and dispatches claimed slots to jobs.
type Scheduler struct {
	slots      SlotRepository
	runs       RunRepository
	dispatcher Dispatcher
	configs    catalog.ConfigurationManager
	locations  catalog.LocationManager
	tx         *dbtx.Runner
	clock      clock.Clock
	logger     *slog.Logger
	metrics    *metrics.Metrics
	cfg        Config
}

// New wires a Scheduler.
func New(slots SlotRepository, runs RunRepository, dispatcher Dispatcher,
	configs catalog.ConfigurationManager, locations catalog.LocationManager,
	tx *dbtx.Runner, clk clock.Clock, logger *slog.Logger, m *metrics.Metrics, cfg Config) *Scheduler {
	if cfg.InstanceID == "" {
		cfg.InstanceID = uuid.NewString()
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 15 * time.Second
	}
	if cfg.LeaseDuration <= 0 {
		cfg.LeaseDuration = 5 * time.Minute
	}
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 8
	}
	if cfg.ClaimBatch <= 0 {
		cfg.ClaimBatch = 10
	}
	return &Scheduler{slots: slots, runs: runs, dispatcher: dispatcher, configs: configs,
		locations: locations, tx: tx, clock: clk, logger: logger, metrics: m, cfg: cfg}
}

// Run starts the scheduler loop until ctx is cancelled (graceful drain).
func (s *Scheduler) Run(ctx context.Context) {
	s.logger.InfoContext(ctx, "scheduler.started",
		slog.String("instance_id", s.cfg.InstanceID),
		slog.Duration("interval", s.cfg.Interval))
	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()
	// Run once immediately, then on each tick.
	s.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			s.logger.InfoContext(ctx, "scheduler.stopped")
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	now := s.clock.Now()
	if err := s.generateSlots(ctx, now); err != nil {
		s.logger.ErrorContext(ctx, "scheduler.generate_failed", slog.String("error", err.Error()))
	}
	slots, err := s.claimDue(ctx, now)
	if err != nil {
		s.logger.ErrorContext(ctx, "scheduler.claim_failed", slog.String("error", err.Error()))
		return
	}
	if len(slots) == 0 {
		return
	}
	sem := make(chan struct{}, s.cfg.MaxConcurrent)
	var wg sync.WaitGroup
	for _, slot := range slots {
		wg.Add(1)
		sem <- struct{}{}
		go func(slot *Slot) {
			defer wg.Done()
			defer func() { <-sem }()
			s.execute(ctx, slot)
		}(slot)
	}
	wg.Wait()
}

// generateSlots creates due slots for each active configuration × active
// location over a short window (current hour + catch-up). Idempotent.
func (s *Scheduler) generateSlots(ctx context.Context, now time.Time) error {
	configs, err := s.configs.ListActiveConfigurations(ctx)
	if err != nil {
		return err
	}
	locations, err := s.locations.ListActiveLocations(ctx)
	if err != nil {
		return err
	}
	if len(configs) == 0 || len(locations) == 0 {
		return nil
	}
	from := now.UTC().Truncate(time.Hour).Add(-time.Hour)
	to := now.UTC().Truncate(time.Hour).Add(time.Hour)

	return s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		for _, cfg := range configs {
			for _, slotTime := range cfg.CollectionSchedule.SlotTimes(from, to) {
				for _, loc := range locations {
					locID := loc.ID
					slot := &Slot{
						ID:                      ids.New(),
						ProviderConfigurationID: cfg.ID,
						JobType:                 JobForecastCollection,
						LocationID:              &locID,
						SlotTime:                slotTime,
						Status:                  SlotDue,
						CreatedAt:               now,
						UpdatedAt:               now,
					}
					if err := s.slots.Generate(ctx, tx, slot); err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
}

func (s *Scheduler) claimDue(ctx context.Context, now time.Time) ([]*Slot, error) {
	var claimed []*Slot
	err := s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		var cerr error
		claimed, cerr = s.slots.ClaimDue(ctx, tx, s.cfg.InstanceID, now, s.cfg.LeaseDuration, s.cfg.ClaimBatch)
		return cerr
	})
	if err != nil {
		return nil, err
	}
	for range claimed {
		s.metrics.SlotsClaimed.WithLabelValues(JobForecastCollection).Inc()
	}
	return claimed, nil
}

// execute runs one claimed slot: start run → dispatch → finish run → update slot.
func (s *Scheduler) execute(ctx context.Context, slot *Slot) {
	start := s.clock.Now()
	log := s.logger.With(slog.String("slot_id", slot.ID.String()), slog.String("job_type", slot.JobType))

	run := &Run{ID: ids.New(), JobType: slot.JobType, SlotID: &slot.ID, StartedAt: start, Status: RunRunning, CreatedAt: start}
	if err := s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error { return s.runs.Start(ctx, tx, run) }); err != nil {
		log.ErrorContext(ctx, "scheduler.run_start_failed", slog.String("error", err.Error()))
		return
	}

	records, err := s.dispatcher.Dispatch(ctx, slot)
	durationMS := int(s.clock.Now().Sub(start).Milliseconds())
	s.metrics.JobDuration.WithLabelValues(slot.JobType).Observe(s.clock.Now().Sub(start).Seconds())

	if err != nil {
		code := classifyError(err)
		_ = s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
			return s.runs.Finish(ctx, tx, run.ID, RunFailed, code, truncateMsg(err.Error()), durationMS, 0)
		})
		attempts := slot.Attempts + 1
		var nextRetry *time.Time
		if attempts < MaxAttempts {
			nr := s.clock.Now().Add(retryBackoff(attempts))
			nextRetry = &nr
		}
		_ = s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
			return s.slots.Fail(ctx, tx, slot.ID, run.ID, attempts, nextRetry)
		})
		log.ErrorContext(ctx, "scheduler.job_failed",
			slog.String("error_code", code), slog.Int("attempts", attempts), slog.String("error", err.Error()))
		return
	}

	_ = s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		return s.runs.Finish(ctx, tx, run.ID, RunCompleted, "", "", durationMS, records)
	})
	_ = s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		return s.slots.Complete(ctx, tx, slot.ID, run.ID)
	})
	log.InfoContext(ctx, "scheduler.job_completed",
		slog.Int("records_affected", records), slog.Int("duration_ms", durationMS))
}

// retryBackoff returns the FC-08 backoff for attempt n (1-based): 1,2,4,8,16s.
func retryBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 5 {
		attempt = 5
	}
	return time.Duration(1<<(attempt-1)) * time.Second
}

func classifyError(err error) string {
	var circuitOpen *domain.CircuitOpenError
	if errors.As(err, &circuitOpen) {
		return "circuit_open"
	}
	if errors.Is(err, domain.ErrInactive) {
		return "inactive"
	}
	return "error"
}

func truncateMsg(s string) string {
	const max = 500
	if len(s) > max {
		return s[:max]
	}
	return s
}
