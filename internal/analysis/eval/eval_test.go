package eval_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/internal/analysis/eval"
)

func f(x float64) *float64 { return &x }

func deref(t *testing.T, p *float64) float64 {
	t.Helper()
	require.NotNil(t, p)
	return *p
}

// TV-1: continuous metrics (methodology §10).
func TestTV1_Continuous(t *testing.T) {
	var c eval.Continuous
	for _, p := range [][2]float64{{15.0, 13.5}, {20.0, 21.0}, {18.0, 18.0}, {25.0, 22.0}} {
		c.Add(p[0], p[1], 1.0)
	}
	assert.InDelta(t, 1.375, deref(t, c.MAE()), 1e-9)
	assert.InDelta(t, 1.75, deref(t, c.RMSE()), 1e-9)
	assert.InDelta(t, 0.875, deref(t, c.Bias()), 1e-9)
	assert.Equal(t, 4, c.N())
}

// TV-2: categorical metrics (methodology §10).
func TestTV2_Categorical(t *testing.T) {
	c := eval.Confusion{TP: 40, FP: 10, FN: 5, TN: 45}
	assert.InDelta(t, 0.8889, deref(t, c.Recall()), 1e-4)
	assert.InDelta(t, 0.8000, deref(t, c.Precision()), 1e-4)
	assert.InDelta(t, 0.8421, deref(t, c.F1()), 1e-4)
	assert.InDelta(t, 0.1818, deref(t, c.FalseAlarmRate()), 1e-4)
	assert.InDelta(t, 0.7273, deref(t, c.ThreatScore()), 1e-4)
	assert.InDelta(t, 0.8500, deref(t, c.OccurrenceAgreement()), 1e-4)
}

// TV-3: zero denominators → null (never 0/NaN), except FAR and agreement which
// have non-zero denominators here (methodology §10).
func TestTV3_ZeroDenominators(t *testing.T) {
	c := eval.Confusion{TN: 100}
	assert.Nil(t, c.Recall())
	assert.Nil(t, c.Precision())
	assert.Nil(t, c.F1())
	assert.InDelta(t, 0.0, deref(t, c.FalseAlarmRate()), 1e-9)
	assert.InDelta(t, 1.0, deref(t, c.OccurrenceAgreement()), 1e-9)
}

// TV-4: Brier score (methodology §10).
func TestTV4_Brier(t *testing.T) {
	var b eval.Brier
	b.Add(0.9, true, 1.0)
	b.Add(0.2, false, 1.0)
	b.Add(0.6, false, 1.0)
	b.Add(0.4, true, 1.0)
	assert.InDelta(t, 0.1925, deref(t, b.Score()), 1e-9)
}

// TV-5: weighted MAE by observation quality (methodology §10).
func TestTV5_WeightedMAE(t *testing.T) {
	var c eval.Continuous
	c.Add(2.0, 0.0, 1.0) // |e| = 2.0, w = 1.0
	c.Add(1.0, 0.0, 0.5) // |e| = 1.0, w = 0.5
	assert.InDelta(t, 1.6667, deref(t, c.MAE()), 1e-4)
}

// Empty accumulators return null, never NaN/±Inf (property 8).
func TestEmptyAccumulators_Null(t *testing.T) {
	var c eval.Continuous
	assert.Nil(t, c.MAE())
	assert.Nil(t, c.RMSE())
	assert.Nil(t, c.Bias())
	var cf eval.Confusion
	assert.Nil(t, cf.Recall())
	assert.Nil(t, cf.Precision())
	assert.Nil(t, cf.F1())
	assert.Nil(t, cf.FalseAlarmRate())
	assert.Nil(t, cf.ThreatScore())
	assert.Nil(t, cf.OccurrenceAgreement())
	var b eval.Brier
	assert.Nil(t, b.Score())
}

// Classification boundaries: forecast rain at prob ≥ 0.5 or amount > 0;
// observed rain at mm ≥ 0.1 (methodology §4.2).
func TestEventBoundaries(t *testing.T) {
	// Forecast rain: probability boundary is inclusive at 0.5.
	assert.True(t, eval.ForecastRain(f(0.5), nil))
	assert.False(t, eval.ForecastRain(f(0.4999), nil))
	// Forecast rain: amount boundary is strict (> 0).
	assert.False(t, eval.ForecastRain(nil, f(0.0)))
	assert.True(t, eval.ForecastRain(nil, f(0.0001)))
	// Either clause triggers; neither present → dry.
	assert.True(t, eval.ForecastRain(f(0.2), f(0.5)))
	assert.False(t, eval.ForecastRain(nil, nil))

	// Observed rain: mm boundary is inclusive at 0.1; trace below is dry.
	assert.True(t, eval.ObservedRain(f(0.1)))
	assert.False(t, eval.ObservedRain(f(0.0999)))
	assert.False(t, eval.ObservedRain(nil))
}

// Eligibility: both sides non-null and not suspect (methodology §3).
func TestEligible(t *testing.T) {
	assert.True(t, eval.Eligible(f(1), f(2), "valid"))
	assert.True(t, eval.Eligible(f(1), f(2), "corrected"))
	assert.False(t, eval.Eligible(nil, f(2), "valid"))
	assert.False(t, eval.Eligible(f(1), nil, "valid"))
	assert.False(t, eval.Eligible(f(1), f(2), "suspect"))
}

// Provenance weights (methodology §6.4).
func TestProvenanceWeight(t *testing.T) {
	assert.Equal(t, 1.0, eval.ProvenanceWeight("station_observation"))
	assert.Equal(t, 0.8, eval.ProvenanceWeight("interpolated"))
	assert.Equal(t, 0.8, eval.ProvenanceWeight("reanalysis"))
	assert.Equal(t, 0.5, eval.ProvenanceWeight("provider_estimated"))
	assert.Equal(t, 0.0, eval.ProvenanceWeight("unknown"))
}
