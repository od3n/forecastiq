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
}

func (f *fakeReadRepo) ListRankings(_ context.Context, _ dbtx.DBTX, _ uuid.UUID, _ int, _ string) ([]*ports.RankingReadRow, error) {
	return f.rankings, nil
}

func (f *fakeReadRepo) LatestObservation(_ context.Context, _ dbtx.DBTX, _ uuid.UUID) (*ports.ObservationContextRow, error) {
	return f.obs, nil
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
