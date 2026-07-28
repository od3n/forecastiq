package eval_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/internal/analysis/eval"
)

// Wilson interval known value: 50/100 → ≈ [0.4038, 0.5962] (methodology §7.4).
func TestWilson_KnownValue(t *testing.T) {
	lo, hi := eval.Wilson(0.5, 100)
	assert.InDelta(t, 0.4038, lo, 1e-3)
	assert.InDelta(t, 0.5962, hi, 1e-3)
}

// Wilson always brackets the point estimate and clamps to [0,1].
func TestWilson_BracketsAndClamps(t *testing.T) {
	rng := rand.New(rand.NewSource(7)) //nolint:gosec // deterministic test fuzz, not security
	for i := 0; i < 2000; i++ {
		p := rng.Float64()
		n := rng.Float64()*200 + 1
		lo, hi := eval.Wilson(p, n)
		assert.GreaterOrEqual(t, lo, 0.0)
		assert.LessOrEqual(t, hi, 1.0)
		// STRICT bracketing (no epsilon): the accuracy_metrics CHECK
		// (ci_lower <= value <= ci_upper) admits no floating-point slack, so
		// neither may this property (WP-26b regression).
		assert.LessOrEqual(t, lo, p)
		assert.GreaterOrEqual(t, hi, p)
	}
	// Boundaries and the degenerate n ≤ 0 case.
	lo0, hi0 := eval.Wilson(0, 10)
	assert.Equal(t, 0.0, lo0)
	assert.GreaterOrEqual(t, hi0, 0.0)
	lo, hi := eval.Wilson(0.5, 0)
	assert.Equal(t, 0.0, lo)
	assert.Equal(t, 0.0, hi)
}

// Regression (WP-26b reliability suite): pHat=0 at small n produced a lower
// bound of ≈ +2.8e-17 — a few ULP above the exact bound — violating the DB
// CHECK (ci_lower <= value) and failing the whole aggregation transaction.
// The FAR cell that surfaced it: p̂=0 over an effective n of 6.
func TestWilson_ZeroPHat_LowerBoundExactlyZero(t *testing.T) {
	for _, n := range []float64{1, 2, 3, 5, 6, 7, 11, 24, 100} {
		lo, hi := eval.Wilson(0, n)
		require.Equal(t, 0.0, lo, "n=%v: lower must be exactly 0 at pHat=0", n)
		require.GreaterOrEqual(t, hi, 0.0)
		lo1, hi1 := eval.Wilson(1, n)
		require.Equal(t, 1.0, hi1, "n=%v: upper must be exactly 1 at pHat=1", n)
		require.LessOrEqual(t, lo1, 1.0)
	}
}

// Continuous CI brackets its point estimate; empty/singleton → nil (no CI).
func TestContinuousCI_BracketsAndNil(t *testing.T) {
	var empty eval.Continuous
	lo, hi := empty.MAECI()
	assert.Nil(t, lo)
	assert.Nil(t, hi)

	var one eval.Continuous
	one.Add(5, 3, 1) // n = 1 → no degrees of freedom
	lo, hi = one.MAECI()
	assert.Nil(t, lo)
	assert.Nil(t, hi)

	var c eval.Continuous
	for _, e := range []float64{1.5, -1.0, 0.0, 3.0} {
		c.Add(e, 0, 1)
	}
	mae := *c.MAE()
	lo, hi = c.MAECI()
	require.NotNil(t, lo)
	require.NotNil(t, hi)
	assert.LessOrEqual(t, *lo, mae)
	assert.GreaterOrEqual(t, *hi, mae)
}

// TestBiasCI_CoverageProbability is the CI-sanity simulation: for normally
// distributed errors, the 95% bias CI should contain the true mean in ~95% of
// trials (methodology §7.4). We allow a generous band to avoid flakiness.
func TestBiasCI_CoverageProbability(t *testing.T) {
	rng := rand.New(rand.NewSource(20260723)) //nolint:gosec // deterministic simulation, not security
	const trials, n, trueMean, sigma = 3000, 40, 2.0, 1.5
	covered := 0
	for i := 0; i < trials; i++ {
		var c eval.Continuous
		for j := 0; j < n; j++ {
			c.Add(trueMean+rng.NormFloat64()*sigma, 0, 1) // error ~ N(trueMean, sigma)
		}
		lo, hi := c.BiasCI()
		require.NotNil(t, lo)
		require.NotNil(t, hi)
		if *lo <= trueMean && trueMean <= *hi {
			covered++
		}
	}
	rate := float64(covered) / trials
	// Normal-approx CI with z=1.96 at n=40: empirical coverage should sit near 0.95.
	assert.InDelta(t, 0.95, rate, 0.03)
	assert.False(t, math.IsNaN(rate))
}
