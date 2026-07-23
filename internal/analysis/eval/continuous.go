package eval

import "math"

// Continuous accumulates weighted continuous-variable errors (temperature, wind
// speed, humidity, pressure, rain amount) and produces MAE, RMSE, and Bias in
// the weighted forms of methodology §4.1/§6.4:
//
//	MAE  = Σ wᵢ|eᵢ| / Σ wᵢ
//	RMSE = √(Σ wᵢ eᵢ² / Σ wᵢ)
//	Bias = Σ wᵢ eᵢ / Σ wᵢ    (eᵢ = fᵢ − oᵢ)
//
// With all weights 1 these reduce to the unweighted definitions (TV-1). The
// accumulator is order-independent (permutation-invariant; property 7).
type Continuous struct {
	sumWeight    float64
	sumWeightAbs float64 // Σ w|e|
	sumWeightErr float64 // Σ w e
	sumWeightSq  float64 // Σ w e²
	n            int
}

// Add includes one eligible pair with the given observation-quality weight.
// Callers weight via ProvenanceWeight; pass 1 for an unweighted computation.
func (c *Continuous) Add(forecast, observed, weight float64) {
	e := forecast - observed
	c.sumWeight += weight
	c.sumWeightAbs += weight * math.Abs(e)
	c.sumWeightErr += weight * e
	c.sumWeightSq += weight * e * e
	c.n++
}

// N is the number of pairs added (for the sample-count / minimum-threshold
// checks in later work packages).
func (c *Continuous) N() int { return c.n }

// SumWeight is the total weight accumulated (Σ wᵢ).
func (c *Continuous) SumWeight() float64 { return c.sumWeight }

// MAE returns the weighted mean absolute error, or nil when no weight has
// accumulated (methodology §5: zero denominator → null).
func (c *Continuous) MAE() *float64 {
	if c.sumWeight == 0 {
		return nil
	}
	return ptr(c.sumWeightAbs / c.sumWeight)
}

// RMSE returns the weighted root mean squared error, or nil when empty.
func (c *Continuous) RMSE() *float64 {
	if c.sumWeight == 0 {
		return nil
	}
	return ptr(math.Sqrt(c.sumWeightSq / c.sumWeight))
}

// Bias returns the weighted signed mean error (positive = forecasts run high),
// or nil when empty.
func (c *Continuous) Bias() *float64 {
	if c.sumWeight == 0 {
		return nil
	}
	return ptr(c.sumWeightErr / c.sumWeight)
}
