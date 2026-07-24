package analysis

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/analysis/domain"
	"github.com/forecastiq/forecastiq/internal/analysis/ports"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
	"github.com/forecastiq/forecastiq/internal/platform/ids"
	"github.com/forecastiq/forecastiq/internal/platform/metrics"
)

// RankService turns accuracy metrics into provider_rankings rows per ranking
// cell (location × horizon × period), using the deterministic cohort engine
// (methodology §6–§7; workflow §4). Each recompute writes new rows atomically
// per cell and supersedes the previous live rows (BR-RANK-07).
type RankService struct {
	repo    ports.RankingRepository
	tx      *dbtx.Runner
	pool    dbtx.DBTX
	metrics *metrics.Metrics
	clock   clock.Clock
	logger  *slog.Logger
}

// NewRankService wires a RankService.
func NewRankService(repo ports.RankingRepository, tx *dbtx.Runner, pool dbtx.DBTX,
	m *metrics.Metrics, clk clock.Clock, logger *slog.Logger) *RankService {
	return &RankService{repo: repo, tx: tx, pool: pool, metrics: m, clock: clk, logger: logger}
}

// RankBatch ranks the current calendar-month period (the trailing evaluation
// window; methodology §7.1). Returns the number of ranking rows written.
func (s *RankService) RankBatch(ctx context.Context) (int, error) {
	now := s.clock.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	return s.RankPeriod(ctx, domain.Period{Kind: domain.PeriodMonthly, Start: start, End: start.AddDate(0, 1, 0)})
}

// RankPeriod ranks every cell that has accuracy metrics for the exact period.
func (s *RankService) RankPeriod(ctx context.Context, period domain.Period) (int, error) {
	cells, err := s.repo.ListRankingCells(ctx, s.pool, period.Start, period.End)
	if err != nil {
		return 0, err
	}
	days := int(period.End.Sub(period.Start).Hours() / 24)
	total := 0
	for _, cell := range cells {
		vals, err := s.repo.ReadCohortMetrics(ctx, s.pool, cell.LocationID, cell.HorizonMinutes, period.Start, period.End)
		if err != nil {
			return total, err
		}
		cohort := buildCohort(vals, days)
		ranked := domain.RankCohort(cohort)
		rows := make([]*ports.RankingRow, 0, len(ranked))
		for _, r := range ranked {
			row, err := s.toRow(r, cell, period)
			if err != nil {
				return total, err
			}
			rows = append(rows, row)
		}
		// Atomic publication for the cell (workflow §4): insert the cohort's new
		// rows, then supersede the previous live rows — readers never see a
		// half-updated cohort.
		if err := s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
			if err := s.repo.InsertRankings(ctx, tx, rows); err != nil {
				return err
			}
			for _, row := range rows {
				if err := s.repo.SupersedePreviousRankings(ctx, tx, row); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return total, err
		}
		total += len(rows)
	}
	if s.metrics != nil {
		s.metrics.RankingsPublished.Add(float64(total))
	}
	s.logger.InfoContext(ctx, "rankings.published", slog.Int("rows", total), slog.Int("cells", len(cells)))
	return total, nil
}

// toRow assembles a persistable ranking row (marshals the component breakdown).
func (s *RankService) toRow(r domain.ProviderRanking, cell ports.LocationHorizon, period domain.Period) (*ports.RankingRow, error) {
	componentJSON, err := json.Marshal(r.ComponentScores)
	if err != nil {
		return nil, err
	}
	return &ports.RankingRow{
		ID:                  ids.New(),
		ProviderID:          r.ProviderID,
		LocationID:          cell.LocationID,
		HorizonMinutes:      cell.HorizonMinutes,
		CompositeScore:      r.CompositeScore,
		CILower:             r.CILower,
		CIUpper:             r.CIUpper,
		Status:              r.Status,
		SampleCount:         r.SampleCount,
		Coverage:            r.Coverage,
		Reliability:         r.Reliability,
		ComponentScoresJSON: componentJSON,
		MethodologyVersion:  domain.MethodologyVersion,
		WeightsVersion:      domain.WeightsVersion,
		HorizonProfile:      domain.ProfileUniform,
		PeriodStart:         period.Start,
		PeriodEnd:           period.End,
	}, nil
}

// buildCohort maps a cell's live accuracy metrics into per-provider ranking
// inputs (methodology §6.1): temp_mae, |temp_bias|, precip_f1, rain_mae_all,
// wind_mae, coverage (min across variables), reliability, plus required-variable
// sample counts.
func buildCohort(vals []ports.MetricValue, calendarDays int) []domain.ProviderInput {
	type acc struct {
		values       map[string]*float64
		ciHalf       map[string]*float64
		tempSample   int
		precipSample int
		coverages    []float64
	}
	provs := map[uuid.UUID]*acc{}
	var order []uuid.UUID
	get := func(id uuid.UUID) *acc {
		a, ok := provs[id]
		if !ok {
			a = &acc{values: map[string]*float64{}, ciHalf: map[string]*float64{}}
			provs[id] = a
			order = append(order, id)
		}
		return a
	}
	halfOf := func(m ports.MetricValue) *float64 {
		if m.CILower == nil || m.CIUpper == nil {
			return nil
		}
		h := (*m.CIUpper - *m.CILower) / 2
		return &h
	}

	for _, m := range vals {
		a := get(m.ProviderID)
		switch {
		case m.Variable == domain.VarTemperature && m.MetricType == domain.MetricMAE:
			a.values["temp_mae"] = m.Value
			a.ciHalf["temp_mae"] = halfOf(m)
			a.tempSample = m.SampleCount
		case m.Variable == domain.VarTemperature && m.MetricType == domain.MetricBias:
			if m.Value != nil {
				abs := math.Abs(*m.Value)
				a.values["temp_bias_abs"] = &abs
			}
			a.ciHalf["temp_bias_abs"] = halfOf(m)
		case m.Variable == domain.VarPrecipitation && m.MetricType == domain.MetricF1:
			a.values["precip_f1"] = m.Value
			a.ciHalf["precip_f1"] = halfOf(m)
			a.precipSample = m.SampleCount
		case m.Variable == domain.VarPrecipitation && m.MetricType == domain.MetricRainMAEAll:
			a.values["rain_mae_all"] = m.Value
			a.ciHalf["rain_mae_all"] = halfOf(m)
		case m.Variable == domain.VarWindSpeed && m.MetricType == domain.MetricMAE:
			a.values["wind_mae"] = m.Value
			a.ciHalf["wind_mae"] = halfOf(m)
		case m.MetricType == domain.MetricCoverage:
			if m.Value != nil {
				a.coverages = append(a.coverages, *m.Value)
			}
		case m.Variable == domain.VarAll && m.MetricType == domain.MetricReliability:
			a.values["reliability"] = m.Value
		}
	}

	out := make([]domain.ProviderInput, 0, len(order))
	for _, id := range order {
		a := provs[id]
		var coverage *float64
		if len(a.coverages) > 0 {
			m := a.coverages[0]
			for _, c := range a.coverages[1:] {
				if c < m {
					m = c
				}
			}
			coverage = &m
		}
		a.values["coverage"] = coverage
		out = append(out, domain.ProviderInput{
			ProviderID:        id,
			Values:            a.values,
			CIHalf:            a.ciHalf,
			TempSampleCount:   a.tempSample,
			PrecipSampleCount: a.precipSample,
			CalendarDays:      calendarDays,
			Coverage:          coverage,
			Reliability:       a.values["reliability"],
		})
	}
	return out
}
