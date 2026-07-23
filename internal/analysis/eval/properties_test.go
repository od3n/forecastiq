package eval_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/internal/analysis/eval"
)

// The property tests fuzz random inputs to assert the invariants in
// methodology §11 (pair-level: properties 1–8). Composite/recompute properties
// (9–11) belong to later work packages.

const fuzzIters = 2000

func newRNG() *rand.Rand { return rand.New(rand.NewSource(42)) } //nolint:gosec // deterministic test fuzz, not security

// Property 1: MAE ≥ 0, and MAE = 0 iff all errors are 0.
// Property 2: RMSE ≥ MAE, with equality iff all |errors| are equal.
// Property 3: |Bias| ≤ RMSE.
func TestProperties_Continuous(t *testing.T) {
	rng := newRNG()
	for i := 0; i < fuzzIters; i++ {
		var c eval.Continuous
		n := rng.Intn(20) + 1
		allZero := true
		firstAbs, sameAbs := -1.0, true
		for j := 0; j < n; j++ {
			e := (rng.Float64() - 0.5) * 40 // [-20, 20)
			w := rng.Float64()*2 + 0.01     // (0.01, 2.01)
			c.Add(e, 0, w)                  // forecast=e, observed=0 → error e
			if e != 0 {
				allZero = false
			}
			if firstAbs < 0 {
				firstAbs = math.Abs(e)
			} else if math.Abs(math.Abs(e)-firstAbs) > 1e-12 {
				sameAbs = false
			}
		}
		mae := deref(t, c.MAE())
		rmse := deref(t, c.RMSE())
		bias := deref(t, c.Bias())

		assert.GreaterOrEqual(t, mae, 0.0)       // P1
		assert.Equal(t, allZero, mae < 1e-12)    // P1 iff
		assert.GreaterOrEqual(t, rmse, mae-1e-9) // P2
		if sameAbs {
			assert.InDelta(t, mae, rmse, 1e-6) // P2 equality
		}
		assert.LessOrEqual(t, math.Abs(bias), rmse+1e-9) // P3
	}
}

// Property 4: 0 ≤ Precision, Recall, F1, FAR, TS ≤ 1 (when non-null).
// Property 5: F1 = 2PR/(P+R) whenever P+R > 0; F1 null iff P and R both null.
func TestProperties_Categorical(t *testing.T) {
	rng := newRNG()
	inUnit := func(t *testing.T, p *float64) {
		if p != nil {
			assert.GreaterOrEqual(t, *p, 0.0)
			assert.LessOrEqual(t, *p, 1.0)
		}
	}
	for i := 0; i < fuzzIters; i++ {
		var c eval.Confusion
		c.TP = rng.Float64() * 50
		c.FP = rng.Float64() * 50
		c.FN = rng.Float64() * 50
		c.TN = rng.Float64() * 50
		// Occasionally zero a cell to exercise null paths.
		if rng.Intn(4) == 0 {
			c.TP = 0
		}
		p, r, f1 := c.Precision(), c.Recall(), c.F1()
		inUnit(t, p)
		inUnit(t, r)
		inUnit(t, f1)
		inUnit(t, c.FalseAlarmRate())
		inUnit(t, c.ThreatScore())

		// P5: relation with Precision/Recall.
		if p != nil && r != nil && (*p+*r) > 0 {
			require.NotNil(t, f1)
			assert.InDelta(t, 2*(*p)*(*r)/(*p+*r), *f1, 1e-9)
		}
		// P5: F1 null iff Precision and Recall both null.
		assert.Equal(t, p == nil && r == nil, f1 == nil)
	}
}

// Property 4 (BS) + Property 8: Brier ∈ [0,1] and never NaN/±Inf.
func TestProperties_Brier(t *testing.T) {
	rng := newRNG()
	for i := 0; i < fuzzIters; i++ {
		var b eval.Brier
		n := rng.Intn(20) + 1
		for j := 0; j < n; j++ {
			b.Add(rng.Float64(), rng.Intn(2) == 1, rng.Float64()*2+0.01)
		}
		bs := deref(t, b.Score())
		assert.GreaterOrEqual(t, bs, 0.0)
		assert.LessOrEqual(t, bs, 1.0)
		assert.False(t, math.IsNaN(bs) || math.IsInf(bs, 0))
	}
}

// Property 6: adding a pair whose |error| equals the current MAE leaves MAE
// unchanged (beyond float ε), for any weight.
func TestProperty6_MAEStability(t *testing.T) {
	rng := newRNG()
	for i := 0; i < fuzzIters; i++ {
		var c eval.Continuous
		n := rng.Intn(20) + 1
		for j := 0; j < n; j++ {
			c.Add((rng.Float64()-0.5)*40, 0, rng.Float64()*2+0.01)
		}
		before := deref(t, c.MAE())
		c.Add(before, 0, rng.Float64()*2+0.01) // error = MAE
		after := deref(t, c.MAE())
		assert.InDelta(t, before, after, 1e-9)
	}
}

// Property 7: metrics are permutation-invariant over pair order.
func TestProperty7_PermutationInvariant(t *testing.T) {
	rng := newRNG()
	type pair struct {
		e, w float64
	}
	for i := 0; i < 500; i++ {
		n := rng.Intn(30) + 1
		pairs := make([]pair, n)
		for j := range pairs {
			pairs[j] = pair{(rng.Float64() - 0.5) * 40, rng.Float64()*2 + 0.01}
		}
		var a eval.Continuous
		for _, p := range pairs {
			a.Add(p.e, 0, p.w)
		}
		shuffled := make([]pair, n)
		copy(shuffled, pairs)
		rng.Shuffle(n, func(x, y int) { shuffled[x], shuffled[y] = shuffled[y], shuffled[x] })
		var b eval.Continuous
		for _, p := range shuffled {
			b.Add(p.e, 0, p.w)
		}
		assert.InDelta(t, deref(t, a.MAE()), deref(t, b.MAE()), 1e-9)
		assert.InDelta(t, deref(t, a.RMSE()), deref(t, b.RMSE()), 1e-9)
		assert.InDelta(t, deref(t, a.Bias()), deref(t, b.Bias()), 1e-9)
	}
}
