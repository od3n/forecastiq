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
	// JobTimeout bounds a single dispatched job. The job runs under a context
	// detached from the scheduler loop so a shutdown signal does not cancel
	// work already in flight (graceful drain; workflow 05 §7).
	JobTimeout time.Duration
	// DrainTimeout bounds how long Run waits for in-flight jobs after the loop
	// context is cancelled before returning (leases on any unfinished job
	// expire naturally and are reclaimed on restart).
	DrainTimeout time.Duration
	// MissedThreshold is how late a claim may be (claimed_at - slot_time)
	// before the slot counts as a missed schedule (watchdog signal).
	MissedThreshold time.Duration
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

	inflight     sync.WaitGroup // tracks dispatched jobs for graceful drain
	sem          chan struct{}  // bounds concurrent jobs (goroutine pool)
	lastProgress time.Time      // last tick that claimed work (watchdog)
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
	if cfg.JobTimeout <= 0 {
		cfg.JobTimeout = 60 * time.Second
	}
	if cfg.DrainTimeout <= 0 {
		cfg.DrainTimeout = 30 * time.Second
	}
	if cfg.MissedThreshold <= 0 {
		if cfg.MissedThreshold = 2 * cfg.Interval; cfg.MissedThreshold < 2*time.Minute {
			cfg.MissedThreshold = 2 * time.Minute
		}
	}
	return &Scheduler{slots: slots, runs: runs, dispatcher: dispatcher, configs: configs,
		locations: locations, tx: tx, clock: clk, logger: logger, metrics: m, cfg: cfg,
		sem: make(chan struct{}, cfg.MaxConcurrent)}
}

// Run starts the scheduler loop until ctx is cancelled, then drains in-flight
// jobs within DrainTimeout before returning (workflow 05 §7).
func (s *Scheduler) Run(ctx context.Context) {
	s.logger.InfoContext(ctx, "scheduler.started",
		slog.String("instance_id", s.cfg.InstanceID),
		slog.Duration("interval", s.cfg.Interval))
	s.lastProgress = s.clock.Now()
	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()
	// Run once immediately, then on each tick.
	s.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			s.drain()
			s.logger.Info("scheduler.stopped")
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// drain waits for in-flight jobs to finish, bounded by DrainTimeout. Jobs that
// do not finish keep running under their detached context; their slot leases
// expire and are reclaimed on the next cycle/restart.
func (s *Scheduler) drain() {
	done := make(chan struct{})
	go func() { s.inflight.Wait(); close(done) }()
	select {
	case <-done:
		s.logger.Info("scheduler.drained")
	case <-time.After(s.cfg.DrainTimeout):
		s.logger.Warn("scheduler.drain_timeout", slog.Duration("deadline", s.cfg.DrainTimeout))
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	now := s.clock.Now()
	if err := s.generateSlots(ctx, now); err != nil {
		s.logger.ErrorContext(ctx, "scheduler.generate_failed", slog.String("error", err.Error()))
	}
	s.watchdog(ctx, now)
	slots, err := s.claimDue(ctx, now)
	if err != nil {
		s.logger.ErrorContext(ctx, "scheduler.claim_failed", slog.String("error", err.Error()))
		return
	}
	if len(slots) == 0 {
		return
	}
	s.lastProgress = now
	// Dispatch each claimed slot on the bounded goroutine pool. Ticks do not
	// block on job completion; the semaphore caps concurrency and slot leases
	// prevent re-claiming in-flight work. Stop launching if the loop is
	// shutting down (drain handles jobs already started).
	for _, slot := range slots {
		select {
		case <-ctx.Done():
			return
		case s.sem <- struct{}{}:
		}
		s.inflight.Add(1)
		go func(slot *Slot) {
			defer s.inflight.Done()
			defer func() { <-s.sem }()
			s.execute(ctx, slot)
		}(slot)
	}
}

// watchdog emits a stall warning when claimable slots exist but no claim has
// made progress for 2× the tick interval (workflow 05 §5: scheduler stalled).
func (s *Scheduler) watchdog(ctx context.Context, now time.Time) {
	if now.Sub(s.lastProgress) < 2*s.cfg.Interval {
		return
	}
	var claimable int
	err := s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		var cerr error
		claimable, cerr = s.slots.CountClaimable(ctx, tx, now)
		return cerr
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "scheduler.watchdog_failed", slog.String("error", err.Error()))
		return
	}
	if claimable > 0 {
		s.logger.WarnContext(ctx, "scheduler.stalled",
			slog.Int("claimable_slots", claimable),
			slog.Duration("since_last_progress", now.Sub(s.lastProgress)))
	}
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
	for _, slot := range claimed {
		s.metrics.SlotsClaimed.WithLabelValues(slot.JobType).Inc()
		// Lag = how long after its scheduled time the slot was claimed. A lag
		// beyond MissedThreshold means the schedule was missed and caught up
		// late (workflow 05 §10).
		lag := slotLag(now, slot.SlotTime)
		s.metrics.SchedulerLag.WithLabelValues(slot.JobType).Observe(lag.Seconds())
		if lag > s.cfg.MissedThreshold {
			s.metrics.MissedSlots.WithLabelValues(slot.JobType).Inc()
		}
	}
	return claimed, nil
}

// slotLag returns how late a claim is relative to the slot's scheduled time
// (never negative — a slot claimed before its time has zero lag).
func slotLag(now, slotTime time.Time) time.Duration {
	if d := now.Sub(slotTime); d > 0 {
		return d
	}
	return 0
}

// execute runs one claimed slot: start run → dispatch → finish run → update slot.
// The job runs under a context detached from the scheduler loop (bounded by
// JobTimeout) so a shutdown signal drains in-flight work rather than cancelling
// it mid-collection.
func (s *Scheduler) execute(ctx context.Context, slot *Slot) {
	jobCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cfg.JobTimeout)
	defer cancel()
	ctx = jobCtx
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
