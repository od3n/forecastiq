package domain_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/internal/analysis/domain"
)

func p(x float64) *float64 { return &x }

// workedInput builds a §8 provider input (precip pairs = temp pairs for the
// example; 30-day span).
func workedInput(id uuid.UUID, tempMAE, biasAbs, f1, rainMAE, windMAE, cov, rel float64, pairs int) domain.ProviderInput {
	return domain.ProviderInput{
		ProviderID: id,
		Values: map[string]*float64{
			"temp_mae": p(tempMAE), "temp_bias_abs": p(biasAbs), "precip_f1": p(f1),
			"rain_mae_all": p(rainMAE), "wind_mae": p(windMAE), "coverage": p(cov), "reliability": p(rel),
		},
		TempSampleCount: pairs, PrecipSampleCount: pairs, CalendarDays: 30,
		Coverage: p(cov), Reliability: p(rel),
	}
}

func composite(t *testing.T, r domain.ProviderRanking) float64 {
	t.Helper()
	require.NotNil(t, r.CompositeScore)
	return *r.CompositeScore
}

func norm(t *testing.T, r domain.ProviderRanking, key string) float64 {
	t.Helper()
	for _, cs := range r.ComponentScores {
		if cs.Component == key {
			require.NotNil(t, cs.Normalized, key)
			return *cs.Normalized
		}
	}
	t.Fatalf("component %s not found", key)
	return 0
}

// TestRankCohort_WorkedExample reproduces methodology §8 (Johor Bahru, +24h, 3
// providers). NOTE: §8's published OM composite (0.940) contains an arithmetic
// slip — the doc's own normalized values and weights sum to ≈0.9568 (DR-06).
// This test asserts the arithmetically correct composites and the §8 ranking
// outcome (order + statuses), which are the substantive acceptance.
func TestRankCohort_WorkedExample(t *testing.T) {
	om := uuid.New()
	ow := uuid.New()
	px := uuid.New()
	cohort := []domain.ProviderInput{
		workedInput(om, 1.20, 0.30, 0.769, 0.90, 1.10, 0.98, 0.99, 720),
		workedInput(ow, 1.50, 0.90, 0.710, 1.40, 1.30, 0.92, 0.97, 700),
		workedInput(px, 1.10, 0.25, 0.682, 0.85, 1.60, 0.55, 0.90, 380),
	}
	res := domain.RankCohort(cohort)
	byID := map[uuid.UUID]domain.ProviderRanking{}
	for _, r := range res {
		byID[r.ProviderID] = r
	}

	// Normalized values match the §8 table (3 dp).
	assert.InDelta(t, 0.917, norm(t, byID[om], "temp_mae"), 1e-3)
	assert.InDelta(t, 0.733, norm(t, byID[ow], "temp_mae"), 1e-3)
	assert.InDelta(t, 1.000, norm(t, byID[px], "temp_mae"), 1e-3)
	assert.InDelta(t, 1.000, norm(t, byID[om], "precip_f1"), 1e-3)
	assert.InDelta(t, 0.923, norm(t, byID[ow], "precip_f1"), 1e-3)
	assert.InDelta(t, 0.887, norm(t, byID[px], "precip_f1"), 1e-3)
	assert.InDelta(t, 0.688, norm(t, byID[px], "wind_mae"), 1e-3)
	assert.InDelta(t, 0.278, norm(t, byID[ow], "temp_bias_abs"), 1e-3)

	// Composites (arithmetically correct; §8 corrected per DR-06).
	assert.InDelta(t, 0.9568, composite(t, byID[om]), 1e-3)
	assert.InDelta(t, 0.7772, composite(t, byID[ow]), 1e-3)
	assert.InDelta(t, 0.6169, composite(t, byID[px]), 1e-3) // 0.8974 × (0.55/0.8)

	// Statuses (§8): OM/OW ranked, PX provisional (coverage 0.55 ∈ [0.5,0.8)).
	assert.Equal(t, domain.StatusRanked, byID[om].Status)
	assert.Equal(t, domain.StatusRanked, byID[ow].Status)
	assert.Equal(t, domain.StatusProvisionally, byID[px].Status)
	assert.Equal(t, 720, byID[om].SampleCount)

	// Rank order OM > OW > PX (no CI → no ties).
	groups := domain.RankOrder(res)
	require.Len(t, groups, 3)
	assert.Equal(t, om, res[groups[0][0]].ProviderID)
	assert.Equal(t, ow, res[groups[1][0]].ProviderID)
	assert.Equal(t, px, res[groups[2][0]].ProviderID)
}

// BR-RANK-04: coverage < 0.5 → unranked (composite nil), can never outrank.
func TestRankCohort_CoverageFloorUnranked(t *testing.T) {
	good := workedInput(uuid.New(), 1.0, 0.1, 0.9, 0.5, 1.0, 0.95, 0.99, 100)
	lowCov := workedInput(uuid.New(), 0.5, 0.05, 0.95, 0.3, 0.8, 0.40, 0.99, 100)
	res := domain.RankCohort([]domain.ProviderInput{good, lowCov})
	for _, r := range res {
		if *r.Coverage < 0.5 {
			assert.Equal(t, domain.StatusUnranked, r.Status)
			assert.Nil(t, r.CompositeScore)
		}
	}
	// The low-coverage provider is excluded from the ranked order entirely.
	groups := domain.RankOrder(res)
	require.Len(t, groups, 1)
	assert.Equal(t, good.ProviderID, res[groups[0][0]].ProviderID)
}

// §7.2 / BR-RANK-02: < 10 pairs → unranked, whether or not coverage is also
// below the 0.5 floor. SampleCount (min of temp/precip pairs) and Coverage
// both survive on the row so the UI attributes the status to samples
// ("Insufficient data (5/30)") rather than the coverage message.
func TestRankCohort_LowSamplesUnranked(t *testing.T) {
	good := workedInput(uuid.New(), 1.0, 0.1, 0.9, 0.5, 1.0, 0.95, 0.99, 100)
	lowBoth := workedInput(uuid.New(), 0.9, 0.08, 0.92, 0.45, 0.9, 0.30, 0.99, 5)    // 5 pairs AND coverage 0.30
	lowSamples := workedInput(uuid.New(), 1.1, 0.20, 0.85, 0.60, 1.2, 0.95, 0.99, 5) // 5 pairs, coverage fine
	res := domain.RankCohort([]domain.ProviderInput{good, lowBoth, lowSamples})
	byID := map[uuid.UUID]domain.ProviderRanking{}
	for _, r := range res {
		byID[r.ProviderID] = r
	}

	for _, in := range []domain.ProviderInput{lowBoth, lowSamples} {
		r := byID[in.ProviderID]
		assert.Equal(t, domain.StatusUnranked, r.Status)
		assert.Nil(t, r.CompositeScore)
		assert.Equal(t, 5, r.SampleCount, "badge count must reflect the low pair count")
		require.NotNil(t, r.Coverage)
		assert.InDelta(t, *in.Coverage, *r.Coverage, 1e-9)
	}

	// Only the good provider participates in the rank order.
	groups := domain.RankOrder(res)
	require.Len(t, groups, 1)
	assert.Equal(t, good.ProviderID, res[groups[0][0]].ProviderID)
}

// Property 9: the coverage penalty is monotonically non-increasing in score as
// coverage falls below 0.8 (methodology §11.9).
func TestRankCohort_PenaltyMonotonic(t *testing.T) {
	score := func(cov float64) float64 {
		// Solo cohort (all norms = 1 for lower/higher-better; direct = value).
		in := workedInput(uuid.New(), 1.0, 0.1, 0.9, 0.5, 1.0, cov, 0.99, 100)
		res := domain.RankCohort([]domain.ProviderInput{in})
		if res[0].CompositeScore == nil {
			return -1 // unranked (cov < 0.5)
		}
		return *res[0].CompositeScore
	}
	prev := score(0.79)
	for cov := 0.78; cov >= 0.50; cov -= 0.02 {
		cur := score(cov)
		assert.LessOrEqual(t, cur, prev+1e-12, "score must not increase as coverage falls")
		prev = cur
	}
}

// Property 10: composite ∈ [0,1] for arbitrary valid cohorts (methodology §11.10).
func TestRankCohort_CompositeBounds(t *testing.T) {
	rng := rand.New(rand.NewSource(14)) //nolint:gosec // deterministic test fuzz, not security
	for i := 0; i < 2000; i++ {
		cohort := make([]domain.ProviderInput, rng.Intn(4)+1)
		for j := range cohort {
			cohort[j] = workedInput(uuid.New(),
				rng.Float64()*5+0.1, rng.Float64()*2, rng.Float64(),
				rng.Float64()*3+0.1, rng.Float64()*4+0.1,
				rng.Float64()*0.5+0.5, rng.Float64()*0.5+0.5, rng.Intn(200))
		}
		for _, r := range domain.RankCohort(cohort) {
			if r.CompositeScore != nil {
				assert.GreaterOrEqual(t, *r.CompositeScore, 0.0)
				assert.LessOrEqual(t, *r.CompositeScore, 1.0)
				assert.False(t, math.IsNaN(*r.CompositeScore))
			}
		}
	}
}

// Tie grouping: providers with overlapping composite CIs share a rank group
// (methodology §7.4 / BR-RANK-05).
func TestRankOrder_TieGrouping(t *testing.T) {
	a := domain.ProviderRanking{ProviderID: uuid.New(), CompositeScore: p(0.90), CILower: p(0.85), CIUpper: p(0.95)}
	b := domain.ProviderRanking{ProviderID: uuid.New(), CompositeScore: p(0.88), CILower: p(0.83), CIUpper: p(0.93)} // overlaps a
	c := domain.ProviderRanking{ProviderID: uuid.New(), CompositeScore: p(0.50), CILower: p(0.45), CIUpper: p(0.55)}
	groups := domain.RankOrder([]domain.ProviderRanking{a, b, c})
	require.Len(t, groups, 2)
	assert.Len(t, groups[0], 2) // a and b tied
	assert.Len(t, groups[1], 1) // c alone
}
