package analysis

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/internal/analysis/domain"
	"github.com/forecastiq/forecastiq/internal/analysis/ports"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// fakeReadRepo is a deterministic in-memory ports.ReadRepository.
type fakeReadRepo struct {
	rankings []*ports.RankingReadRow
	obs      *ports.ObservationContextRow
	metrics  []*ports.MetricRow
	statuses map[uuid.UUID]ports.ProviderStatus
	windows  map[uuid.UUID]ports.CollectionWindow
	cells    []*ports.ProviderSummaryCell
	trends   []*ports.TrendBucket
	points   []*ports.ForecastPoint
	compObs  []*ports.ComparisonObservation
}

func (f *fakeReadRepo) ListRankings(_ context.Context, _ dbtx.DBTX, _ uuid.UUID, _ int, _ string) ([]*ports.RankingReadRow, error) {
	return f.rankings, nil
}

func (f *fakeReadRepo) LatestObservation(_ context.Context, _ dbtx.DBTX, _ uuid.UUID) (*ports.ObservationContextRow, error) {
	return f.obs, nil
}

func (f *fakeReadRepo) LocationMetrics(_ context.Context, _ dbtx.DBTX, _ uuid.UUID, _ int) ([]*ports.MetricRow, error) {
	return f.metrics, nil
}

func (f *fakeReadRepo) LocationProviderStatuses(_ context.Context, _ dbtx.DBTX, _ uuid.UUID, _ int) (map[uuid.UUID]ports.ProviderStatus, error) {
	return f.statuses, nil
}

func (f *fakeReadRepo) LocationWindows(_ context.Context, _ dbtx.DBTX, _ uuid.UUID) (map[uuid.UUID]ports.CollectionWindow, error) {
	return f.windows, nil
}

func (f *fakeReadRepo) ProviderRankingCells(_ context.Context, _ dbtx.DBTX, _ uuid.UUID) ([]*ports.ProviderSummaryCell, error) {
	return f.cells, nil
}

func (f *fakeReadRepo) ProviderWindows(_ context.Context, _ dbtx.DBTX, _ uuid.UUID) (map[uuid.UUID]ports.CollectionWindow, error) {
	return f.windows, nil
}

func (f *fakeReadRepo) AccuracyTrends(_ context.Context, _ dbtx.DBTX, _ ports.TrendFilter) ([]*ports.TrendBucket, error) {
	return f.trends, nil
}

func (f *fakeReadRepo) ForecastComparisonPoints(_ context.Context, _ dbtx.DBTX, _ uuid.UUID, _ []uuid.UUID, _ string, _ int, _, _ time.Time) ([]*ports.ForecastPoint, error) {
	return f.points, nil
}

func (f *fakeReadRepo) ComparisonObservations(_ context.Context, _ dbtx.DBTX, _ uuid.UUID, _ string, _, _ time.Time) ([]*ports.ComparisonObservation, error) {
	return f.compObs, nil
}

func fp(x float64) *float64 { return &x }

func row(name, status string, composite float64, scored bool) *ports.RankingReadRow {
	r := &ports.RankingReadRow{
		ProviderID: uuid.New(), ProviderName: name, ProviderSlug: name,
		Status: status, SampleCount: 720,
		MethodologyVersion: domain.MethodologyVersion, WeightsVersion: domain.WeightsVersion,
		HorizonProfile: domain.ProfileUniform,
		PeriodStart:    time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:      time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		CalculatedAt:   time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC),
	}
	if scored {
		r.CompositeScore = fp(composite)
	}
	return r
}

func TestReadService_RankingsOrdering(t *testing.T) {
	// Intentionally shuffled input: PX (provisional), OM (ranked, best),
	// unknown (unranked), OW (ranked, second).
	repo := &fakeReadRepo{
		rankings: []*ports.RankingReadRow{
			row("ProviderX", domain.StatusProvisionally, 0.617, true),
			row("Open-Meteo", domain.StatusRanked, 0.957, true),
			row("Nodata", domain.StatusUnranked, 0, false),
			row("OpenWeather", domain.StatusRanked, 0.777, true),
		},
		obs: &ports.ObservationContextRow{ObservedAt: time.Now().UTC(), Source: "openmeteo_historical", ObservationType: "reanalysis", QualityFlag: "valid"},
	}
	svc := NewReadService(repo, nil)
	res, err := svc.Rankings(context.Background(), RankingsQuery{LocationID: uuid.New(), HorizonMinutes: 1440})
	require.NoError(t, err)
	require.True(t, res.HasRows)
	require.Len(t, res.Rows, 4)

	// ranked (best→second) first, then provisional, then unranked (rank 0).
	assert.Equal(t, "Open-Meteo", res.Rows[0].Row.ProviderName)
	assert.Equal(t, 1, res.Rows[0].Rank)
	assert.Equal(t, "OpenWeather", res.Rows[1].Row.ProviderName)
	assert.Equal(t, 2, res.Rows[1].Rank)
	assert.Equal(t, "ProviderX", res.Rows[2].Row.ProviderName)
	assert.Equal(t, 3, res.Rows[2].Rank, "provisional continues the numbering after ranked")
	assert.Equal(t, "Nodata", res.Rows[3].Row.ProviderName)
	assert.Equal(t, 0, res.Rows[3].Rank, "unranked is unordered (rank 0)")

	assert.Equal(t, domain.MethodologyVersion, res.MethodologyVersion)
	assert.Equal(t, DefaultMinSampleCount, res.MinSampleCount)
	require.NotNil(t, res.LastCalculatedAt)
	require.NotNil(t, res.Observation)
}

func TestReadService_TieGrouping(t *testing.T) {
	a := row("A", domain.StatusRanked, 0.90, true)
	a.CILower, a.CIUpper = fp(0.85), fp(0.95)
	b := row("B", domain.StatusRanked, 0.88, true)
	b.CILower, b.CIUpper = fp(0.83), fp(0.93) // overlaps A → tied at rank 1
	c := row("C", domain.StatusRanked, 0.60, true)
	c.CILower, c.CIUpper = fp(0.55), fp(0.65) // separate → rank 2
	svc := NewReadService(&fakeReadRepo{rankings: []*ports.RankingReadRow{a, b, c}}, nil)

	res, err := svc.Rankings(context.Background(), RankingsQuery{LocationID: uuid.New(), HorizonMinutes: 1440})
	require.NoError(t, err)
	byName := map[string]RankedRow{}
	for _, r := range res.Rows {
		byName[r.Row.ProviderName] = r
	}
	assert.Equal(t, 1, byName["A"].Rank)
	assert.True(t, byName["A"].Tied)
	assert.Equal(t, 1, byName["B"].Rank)
	assert.True(t, byName["B"].Tied)
	assert.Equal(t, 2, byName["C"].Rank)
	assert.False(t, byName["C"].Tied)
}

func TestReadService_NoRows(t *testing.T) {
	svc := NewReadService(&fakeReadRepo{}, nil)
	res, err := svc.Rankings(context.Background(), RankingsQuery{LocationID: uuid.New(), HorizonMinutes: 1440})
	require.NoError(t, err)
	assert.False(t, res.HasRows)
	assert.Empty(t, res.Rows)
	assert.Nil(t, res.LastCalculatedAt)
}

func TestMethodology_ConsistentWithEngine(t *testing.T) {
	m := domain.Methodology()
	assert.Equal(t, domain.MethodologyVersion, m.MethodologyVersion)
	assert.Equal(t, domain.WeightsVersion, m.WeightsVersion)

	// default_weights mirror the engine's components and sum to 1.0.
	require.Len(t, m.DefaultWeights, len(domain.Components))
	sum := 0.0
	for _, w := range m.DefaultWeights {
		sum += w.Weight
	}
	assert.InDelta(t, 1.0, sum, 1e-9)

	assert.Equal(t, 30, m.Thresholds["ranked"])
	assert.Equal(t, 10, m.Thresholds["provisional"])
	assert.Equal(t, 0.8, m.CoveragePenalty.NoPenaltyAtOrAbove)
	assert.Equal(t, 0.5, m.CoveragePenalty.UnrankedBelow)
	assert.Len(t, m.Statuses, 3)
	assert.NotEmpty(t, m.Formulas)
	assert.NotEmpty(t, m.TieRule)
}

func TestReadService_LocationSummary(t *testing.T) {
	omID := uuid.New()
	repo := &fakeReadRepo{
		metrics: []*ports.MetricRow{
			{ProviderID: omID, Variable: "temperature", MetricType: "mae", Value: fp(1.2), SampleCount: 720},
			{ProviderID: omID, Variable: "precipitation", MetricType: "f1", Value: fp(0.769), SampleCount: 700},
		},
		statuses: map[uuid.UUID]ports.ProviderStatus{omID: {RankingStatus: "ranked", Coverage: fp(0.98), Reliability: fp(0.99)}},
		windows:  map[uuid.UUID]ports.CollectionWindow{omID: {FirstSnapshotAt: tp(2026, 6, 1), LastSnapshotAt: tp(2026, 7, 22)}},
	}
	svc := NewReadService(repo, nil)
	res, err := svc.LocationSummary(context.Background(), uuid.New(), 1440)
	require.NoError(t, err)
	require.True(t, res.HasData)
	require.Len(t, res.Providers, 1)
	p := res.Providers[0]
	assert.Equal(t, "ranked", p.RankingStatus)
	assert.Len(t, p.Metrics, 2)
	// coverage/reliability fold onto the window from the ranking status.
	require.NotNil(t, p.Window.Coverage)
	assert.InDelta(t, 0.98, *p.Window.Coverage, 1e-9)
	require.NotNil(t, res.LastSnapshotAt)
}

func TestReadService_ProviderSummary(t *testing.T) {
	locID := uuid.New()
	repo := &fakeReadRepo{
		cells: []*ports.ProviderSummaryCell{
			{LocationID: locID, LocationName: "Johor Bahru", HorizonMinutes: 1440, CompositeScore: fp(0.957), RankingStatus: "ranked", SampleCount: 720, Coverage: fp(0.98)},
		},
		windows: map[uuid.UUID]ports.CollectionWindow{locID: {FirstSnapshotAt: tp(2026, 6, 1), LastSnapshotAt: tp(2026, 7, 22)}},
	}
	svc := NewReadService(repo, nil)
	res, err := svc.ProviderSummary(context.Background(), uuid.New())
	require.NoError(t, err)
	require.True(t, res.HasData)
	require.Len(t, res.Cells, 1)
	assert.Equal(t, "Johor Bahru", res.Cells[0].LocationName)
}

func TestReadService_TrendsGroupsByProvider(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	repo := &fakeReadRepo{trends: []*ports.TrendBucket{
		{ProviderID: a, PeriodStart: mustDay(2026, 7, 1), PeriodEnd: mustDay(2026, 7, 2), Value: fp(1.1), SampleCount: 24},
		{ProviderID: a, PeriodStart: mustDay(2026, 7, 2), PeriodEnd: mustDay(2026, 7, 3), SampleCount: 0}, // hollow point
		{ProviderID: b, PeriodStart: mustDay(2026, 7, 1), PeriodEnd: mustDay(2026, 7, 2), Value: fp(1.4), SampleCount: 20},
	}}
	svc := NewReadService(repo, nil)
	res, err := svc.Trends(context.Background(), ports.TrendFilter{Aggregation: "daily"}, time.UTC)
	require.NoError(t, err)
	require.Len(t, res.Series, 2)
	assert.Equal(t, a, res.Series[0].ProviderID)
	assert.Len(t, res.Series[0].Buckets, 2)
	// hollow point preserved: bucket with sample_count 0 and nil value retained.
	assert.Nil(t, res.Series[0].Buckets[1].Value)
	assert.Equal(t, 0, res.Series[0].Buckets[1].SampleCount)
	require.NotNil(t, res.LastPeriodEnd)
}

func tp(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

func mustDay(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestTruncToBucket_TZAlignedAndDST(t *testing.T) {
	kl, err := time.LoadLocation("Asia/Kuala_Lumpur") // UTC+8, no DST
	require.NoError(t, err)
	// A UTC-day row maps to the tz day its UTC start falls in; the bucket is the
	// tz-local midnight window (24 h for a non-DST zone).
	start, end := truncToBucket(mustDay(2026, 7, 21), kl, "daily")
	assert.Equal(t, time.Date(2026, 7, 20, 16, 0, 0, 0, time.UTC), start.UTC()) // 07-21 00:00 +08
	assert.Equal(t, 24*time.Hour, end.Sub(start))

	ny, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	// Spring-forward day (2026-03-08): the tz "day" is 23 h long.
	spring := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC) // 07:00 EST, local 2026-03-08
	sStart, sEnd := truncToBucket(spring, ny, "daily")
	assert.Equal(t, 2026, sStart.In(ny).Year())
	assert.Equal(t, 8, sStart.In(ny).Day())
	assert.Equal(t, 0, sStart.In(ny).Hour())
	assert.Equal(t, 23*time.Hour, sEnd.Sub(sStart), "spring-forward day is 23h")

	// Fall-back day (2026-11-01): the tz "day" is 25 h long.
	fall := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	fStart, fEnd := truncToBucket(fall, ny, "daily")
	assert.Equal(t, 25*time.Hour, fEnd.Sub(fStart), "fall-back day is 25h")

	// Monthly + weekly (Monday-start) alignment.
	mStart, mEnd := truncToBucket(time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC), kl, "monthly")
	assert.Equal(t, 1, mStart.In(kl).Day())
	assert.Equal(t, time.August, mEnd.In(kl).Month())
	wStart, _ := truncToBucket(time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC), kl, "weekly") // Thu → Mon 07-20
	assert.Equal(t, time.Monday, wStart.In(kl).Weekday())
}

func TestReadService_TrendsTZBucketing(t *testing.T) {
	kl, err := time.LoadLocation("Asia/Kuala_Lumpur")
	require.NoError(t, err)
	prov := uuid.New()
	// Two consecutive UTC daily rows; in UTC+8 they remain distinct tz days but
	// the bucket boundaries shift to local midnights (07-20 16:00Z / 07-21 16:00Z).
	repo := &fakeReadRepo{trends: []*ports.TrendBucket{
		{ProviderID: prov, PeriodStart: mustDay(2026, 7, 21), PeriodEnd: mustDay(2026, 7, 22), Value: fp(1.2), SampleCount: 24},
		{ProviderID: prov, PeriodStart: mustDay(2026, 7, 22), PeriodEnd: mustDay(2026, 7, 23), Value: fp(1.4), SampleCount: 24},
	}}
	svc := NewReadService(repo, nil)
	res, err := svc.Trends(context.Background(), ports.TrendFilter{Aggregation: "daily"}, kl)
	require.NoError(t, err)
	require.Len(t, res.Series, 1)
	buckets := res.Series[0].Buckets
	require.Len(t, buckets, 2) // distinct tz days, not collapsed
	assert.Equal(t, time.Date(2026, 7, 20, 16, 0, 0, 0, time.UTC), buckets[0].PeriodStart)
	assert.Equal(t, 0, buckets[0].PeriodStart.In(kl).Hour()) // tz-local midnight
	require.NotNil(t, buckets[0].Value)
	assert.InDelta(t, 1.2, *buckets[0].Value, 1e-9) // 1:1 relabel preserves value
}

func TestReadService_ForecastComparison(t *testing.T) {
	prov := uuid.New()
	h0 := mustDay(2026, 7, 21)
	h1 := h0.Add(1 * time.Hour)
	h2 := h0.Add(2 * time.Hour)
	issued := h0.AddDate(0, 0, -1)
	repo := &fakeReadRepo{
		points: []*ports.ForecastPoint{
			{ProviderID: prov, TargetTime: h0, IssuedAt: issued, HorizonMinutes: 1440, Value: 30.0},
			{ProviderID: prov, TargetTime: h1, IssuedAt: issued.Add(time.Hour), HorizonMinutes: 1440, Value: 32.0},
			// h2 has a forecast but NO observation → contributes to the line, not metrics.
			{ProviderID: prov, TargetTime: h2, IssuedAt: issued.Add(2 * time.Hour), HorizonMinutes: 1440, Value: 33.0},
		},
		compObs: []*ports.ComparisonObservation{
			{ObservedAt: h0, Value: 31.0, Source: "openmeteo_historical", ObservationType: "reanalysis", QualityFlag: "valid"},
			{ObservedAt: h1, Value: 31.0, Source: "openmeteo_historical", ObservationType: "reanalysis", QualityFlag: "valid"},
			// h2 observation is absent (gap) — never interpolated.
		},
	}
	svc := NewReadService(repo, nil)
	res, err := svc.ForecastComparison(context.Background(), ComparisonQuery{
		LocationID: uuid.New(), ProviderIDs: []uuid.UUID{prov}, Variable: "temperature", HorizonMinutes: 1440,
	})
	require.NoError(t, err)
	require.Len(t, res.Series, 1)
	assert.Len(t, res.Series[0].Points, 3)          // full forecast line incl. the gap hour
	assert.Equal(t, issued, res.Series[0].IssuedAt) // earliest issuance
	require.Len(t, res.Observations, 2)
	assert.True(t, res.ObservationsAvailable)

	require.Len(t, res.DayMetrics, 1)
	dm := res.DayMetrics[0]
	assert.Equal(t, 2, dm.SampleCount) // only the two matched hours (h2 gap excluded)
	require.NotNil(t, dm.MAE)
	// |30-31| and |32-31| = 1 and 1 → MAE 1.0 (reanalysis weight cancels).
	assert.InDelta(t, 1.0, *dm.MAE, 1e-9)
	require.NotNil(t, dm.Bias)
	assert.InDelta(t, 0.0, *dm.Bias, 1e-9) // (-1 + +1)/2
	require.NotNil(t, res.ErrorBandMAE)
	assert.InDelta(t, 1.0, *res.ErrorBandMAE, 1e-9)
	assert.InDelta(t, 1.0, res.ProvenanceMix["reanalysis"], 1e-9)
	require.NotNil(t, res.LatestObservedAt)
}

func TestReadService_ForecastComparison_NoData(t *testing.T) {
	svc := NewReadService(&fakeReadRepo{}, nil)
	res, err := svc.ForecastComparison(context.Background(), ComparisonQuery{
		LocationID: uuid.New(), ProviderIDs: []uuid.UUID{uuid.New()}, Variable: "temperature", HorizonMinutes: 1440,
	})
	require.NoError(t, err)
	assert.Empty(t, res.Series)
	assert.False(t, res.ObservationsAvailable)
	assert.Nil(t, res.ErrorBandMAE)
	assert.Nil(t, res.LatestObservedAt)
}
