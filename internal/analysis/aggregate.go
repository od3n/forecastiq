package analysis

import (
	"context"
	"log/slog"
	"time"

	"github.com/forecastiq/forecastiq/internal/analysis/domain"
	"github.com/forecastiq/forecastiq/internal/analysis/eval"
	"github.com/forecastiq/forecastiq/internal/analysis/ports"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
	"github.com/forecastiq/forecastiq/internal/platform/ids"
	"github.com/forecastiq/forecastiq/internal/platform/metrics"
)

// AggregateService turns matched pairs into AccuracyMetric rows per cell-period
// (workflow §3; methodology §4/§5/§6.4/§7.4). Each recompute writes new rows and
// supersedes the previous live rows for the same logical key.
type AggregateService struct {
	repo    ports.MetricRepository
	tx      *dbtx.Runner
	pool    dbtx.DBTX
	metrics *metrics.Metrics
	clock   clock.Clock
	logger  *slog.Logger
}

// NewAggregateService wires an AggregateService.
func NewAggregateService(repo ports.MetricRepository, tx *dbtx.Runner, pool dbtx.DBTX,
	m *metrics.Metrics, clk clock.Clock, logger *slog.Logger) *AggregateService {
	return &AggregateService{repo: repo, tx: tx, pool: pool, metrics: m, clock: clk, logger: logger}
}

// AggregateBatch recomputes the standard rolling period set (current day, week,
// month) over every active cell. Returns the number of metric rows written.
func (s *AggregateService) AggregateBatch(ctx context.Context) (int, error) {
	total := 0
	for _, p := range standardPeriods(s.clock.Now().UTC()) {
		n, err := s.AggregatePeriod(ctx, p)
		if err != nil {
			return total, err
		}
		total += n
	}
	if s.metrics != nil {
		s.metrics.MetricRowsWritten.Add(float64(total))
	}
	s.logger.InfoContext(ctx, "aggregation.batch_completed", slog.Int("rows_written", total))
	return total, nil
}

// AggregatePeriod recomputes all active cells for one period.
func (s *AggregateService) AggregatePeriod(ctx context.Context, period domain.Period) (int, error) {
	cells, err := s.repo.ListCells(ctx, s.pool, period.Start, period.End)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, cell := range cells {
		n, err := s.aggregateCellPeriod(ctx, cell, period)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

// aggregateCellPeriod computes every metric row for one cell-period and writes
// them, superseding the previous live rows, in a single transaction.
func (s *AggregateService) aggregateCellPeriod(ctx context.Context, cell domain.Cell, period domain.Period) (int, error) {
	pairs, err := s.repo.ReadPairs(ctx, s.pool, cell, period.Start, period.End)
	if err != nil {
		return 0, err
	}
	counts, err := s.repo.SnapshotVariableCounts(ctx, s.pool, cell, period.Start, period.End)
	if err != nil {
		return 0, err
	}
	scheduled, err := s.repo.ScheduledForecastSlots(ctx, s.pool, cell.ProviderID, cell.LocationID, period.Start, period.End)
	if err != nil {
		return 0, err
	}
	success, err := s.repo.SuccessfulCollections(ctx, s.pool, cell.ProviderID, cell.LocationID, period.Start, period.End)
	if err != nil {
		return 0, err
	}

	rows := s.computeRows(cell, period, pairs, counts, scheduled, success)
	if len(rows) == 0 {
		return 0, nil
	}
	if err := s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		if err := s.repo.InsertMetrics(ctx, tx, rows); err != nil {
			return err
		}
		for _, m := range rows {
			if err := s.repo.SupersedePrevious(ctx, tx, m); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return 0, err
	}
	return len(rows), nil
}

// computeRows runs the evaluation kernel over the pairs and assembles every
// metric row for the cell-period (pure; no I/O).
func (s *AggregateService) computeRows(cell domain.Cell, period domain.Period, pairs []*domain.PairRecord,
	counts ports.VariableCounts, scheduled, success int) []*domain.AccuracyMetric {

	var temp, wind, humidity, pressure, rainAll, rainWet eval.Continuous
	var occ eval.Confusion
	var brier eval.Brier
	occN := 0

	for _, p := range pairs {
		w := eval.ProvenanceWeight(p.ObservationType)
		if eval.Eligible(p.FTemperature, p.OTemperature, p.QualityFlag) {
			temp.Add(*p.FTemperature, *p.OTemperature, w)
		}
		if eval.Eligible(p.FWindSpeed, p.OWindSpeed, p.QualityFlag) {
			wind.Add(*p.FWindSpeed, *p.OWindSpeed, w)
		}
		if eval.Eligible(p.FHumidity, p.OHumidity, p.QualityFlag) {
			humidity.Add(*p.FHumidity, *p.OHumidity, w)
		}
		if eval.Eligible(p.FPressure, p.OPressure, p.QualityFlag) {
			pressure.Add(*p.FPressure, *p.OPressure, w)
		}
		if eval.Eligible(p.FPrecipMM, p.OPrecipMM, p.QualityFlag) {
			rainAll.Add(*p.FPrecipMM, *p.OPrecipMM, w)
			if *p.OPrecipMM >= eval.ObservedRainThresholdMM {
				rainWet.Add(*p.FPrecipMM, *p.OPrecipMM, w)
			}
		}
		// Occurrence needs a known observed state; forecast side is nil-tolerant.
		if p.OPrecipMM != nil && p.QualityFlag != eval.QualitySuspect {
			observedRain := eval.ObservedRain(p.OPrecipMM)
			occ.Add(eval.ForecastRain(p.FPrecipProb, p.FPrecipMM), observedRain, w)
			occN++
			if p.FPrecipProb != nil {
				brier.Add(*p.FPrecipProb, observedRain, w)
			}
		}
	}

	var rows []*domain.AccuracyMetric
	rows = append(rows, s.continuousRows(cell, period, domain.VarTemperature, &temp)...)
	rows = append(rows, s.continuousRows(cell, period, domain.VarWindSpeed, &wind)...)
	rows = append(rows, s.continuousRows(cell, period, domain.VarHumidity, &humidity)...)
	rows = append(rows, s.continuousRows(cell, period, domain.VarPressure, &pressure)...)

	// Rain amount (two accumulators; methodology §4.1).
	rMaeLo, rMaeHi := rainAll.MAECI()
	rows = append(rows, s.metric(cell, period, domain.VarPrecipitation, domain.MetricRainMAEAll, rainAll.MAE(), rMaeLo, rMaeHi, rainAll.N()))
	rwLo, rwHi := rainWet.MAECI()
	rows = append(rows, s.metric(cell, period, domain.VarPrecipitation, domain.MetricRainMAEWet, rainWet.MAE(), rwLo, rwHi, rainWet.N()))

	// Categorical occurrence (Wilson CIs on the metric-specific denominators).
	rows = append(rows,
		s.ratioRow(cell, period, domain.MetricRecall, occ.Recall(), occ.TP+occ.FN, occN),
		s.ratioRow(cell, period, domain.MetricPrecision, occ.Precision(), occ.TP+occ.FP, occN),
		s.ratioRow(cell, period, domain.MetricF1, occ.F1(), 2*occ.TP+occ.FP+occ.FN, occN),
		s.ratioRow(cell, period, domain.MetricFAR, occ.FalseAlarmRate(), occ.FP+occ.TN, occN),
		s.ratioRow(cell, period, domain.MetricThreatScore, occ.ThreatScore(), occ.TP+occ.FP+occ.FN, occN),
		s.ratioRow(cell, period, domain.MetricOccurrenceAgreement, occ.OccurrenceAgreement(), occ.N(), occN),
	)

	// Probabilistic (normal-approx CI).
	bLo, bHi := brier.ScoreCI()
	rows = append(rows, s.metric(cell, period, domain.VarPrecipitation, domain.MetricBrier, brier.Score(), bLo, bHi, occN))

	// Coverage per variable + collection reliability (methodology §4.4).
	rows = append(rows,
		s.ratelessRow(cell, period, domain.VarTemperature, domain.MetricCoverage, counts.Temperature, scheduled),
		s.ratelessRow(cell, period, domain.VarWindSpeed, domain.MetricCoverage, counts.WindSpeed, scheduled),
		s.ratelessRow(cell, period, domain.VarHumidity, domain.MetricCoverage, counts.Humidity, scheduled),
		s.ratelessRow(cell, period, domain.VarPressure, domain.MetricCoverage, counts.Pressure, scheduled),
		s.ratelessRow(cell, period, domain.VarPrecipitation, domain.MetricCoverage, counts.Precipitation, scheduled),
		s.ratelessRow(cell, period, domain.VarAll, domain.MetricReliability, success, scheduled),
	)
	return rows
}

// continuousRows builds the mae/rmse/bias rows for one continuous variable.
func (s *AggregateService) continuousRows(cell domain.Cell, period domain.Period, variable string, c *eval.Continuous) []*domain.AccuracyMetric {
	n := c.N()
	maeLo, maeHi := c.MAECI()
	rmseLo, rmseHi := c.RMSECI()
	biasLo, biasHi := c.BiasCI()
	return []*domain.AccuracyMetric{
		s.metric(cell, period, variable, domain.MetricMAE, c.MAE(), maeLo, maeHi, n),
		s.metric(cell, period, variable, domain.MetricRMSE, c.RMSE(), rmseLo, rmseHi, n),
		s.metric(cell, period, variable, domain.MetricBias, c.Bias(), biasLo, biasHi, n),
	}
}

// ratioRow builds a categorical metric row with a Wilson CI (value nil → null row).
func (s *AggregateService) ratioRow(cell domain.Cell, period domain.Period, metricType string, value *float64, denom float64, n int) *domain.AccuracyMetric {
	if value == nil {
		return s.metric(cell, period, domain.VarPrecipitation, metricType, nil, nil, nil, 0)
	}
	lo, hi := eval.Wilson(*value, denom)
	return s.metric(cell, period, domain.VarPrecipitation, metricType, value, &lo, &hi, n)
}

// ratelessRow builds a coverage/reliability ratio row (no CI). numerator/denom;
// null when denom = 0. Clamped to [0, 1] (methodology §4.4).
func (s *AggregateService) ratelessRow(cell domain.Cell, period domain.Period, variable, metricType string, numerator, denom int) *domain.AccuracyMetric {
	if denom <= 0 {
		return s.metric(cell, period, variable, metricType, nil, nil, nil, 0)
	}
	v := float64(numerator) / float64(denom)
	if v > 1 {
		v = 1
	}
	return s.metric(cell, period, variable, metricType, &v, nil, nil, denom)
}

// metric assembles one row, enforcing the methodology §5 null coupling
// (value NULL ⇔ sample_count 0; CI dropped when value is null).
func (s *AggregateService) metric(cell domain.Cell, period domain.Period, variable, metricType string,
	value, ciLo, ciHi *float64, n int) *domain.AccuracyMetric {
	m := &domain.AccuracyMetric{
		ID:                 ids.New(),
		ProviderID:         cell.ProviderID,
		LocationID:         cell.LocationID,
		HorizonMinutes:     cell.HorizonMinutes,
		Variable:           variable,
		MetricType:         metricType,
		MethodologyVersion: domain.MethodologyVersion,
		PeriodStart:        period.Start,
		PeriodEnd:          period.End,
	}
	if value == nil {
		return m // value/ci nil, sample_count 0
	}
	m.Value, m.CILower, m.CIUpper, m.SampleCount = value, ciLo, ciHi, n
	return m
}

// standardPeriods returns the current day, ISO week (Monday-based), and calendar
// month windows in UTC (workflow §3: daily/weekly/monthly).
func standardPeriods(now time.Time) []domain.Period {
	now = now.UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	// Monday-based week start.
	weekday := int(dayStart.Weekday()) // Sunday=0
	back := (weekday + 6) % 7          // days since Monday
	weekStart := dayStart.AddDate(0, 0, -back)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	return []domain.Period{
		{Kind: domain.PeriodDaily, Start: dayStart, End: dayStart.AddDate(0, 0, 1)},
		{Kind: domain.PeriodWeekly, Start: weekStart, End: weekStart.AddDate(0, 0, 7)},
		{Kind: domain.PeriodMonthly, Start: monthStart, End: monthStart.AddDate(0, 1, 0)},
	}
}
