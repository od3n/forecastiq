package eval

import "math"

// Z95 is the standard-normal quantile for a two-sided 95% interval
// (methodology §7.4 specifies 1.96 literally).
const Z95 = 1.96

// Wilson returns the 95% Wilson score interval for a proportion pHat observed
// over an effective sample size n (methodology §7.4: ratio metrics use the
// Wilson interval). Bounds are clamped to [0, 1]. With fractional (weighted)
// counts, n is the metric's weighted denominator (effective sample size). For
// n ≤ 0 it returns (0, 0); callers treat a zero-denominator metric as null.
func Wilson(pHat, n float64) (lower, upper float64) {
	if n <= 0 {
		return 0, 0
	}
	z2 := Z95 * Z95
	denom := 1 + z2/n
	center := (pHat + z2/(2*n)) / denom
	margin := (Z95 * math.Sqrt(pHat*(1-pHat)/n+z2/(4*n*n))) / denom
	lower = center - margin
	upper = center + margin
	if lower < 0 {
		lower = 0
	}
	if upper > 1 {
		upper = 1
	}
	return lower, upper
}

// ciFromMoments builds a ±Z95·s/√n interval centred on mean, where s is the
// frequency-weight-unbiased sample standard deviation of the underlying
// quantity x (weighted variance = (Σwx² − (Σwx)²/Σw) / (Σw − Σw²/Σw); this
// reduces to the ordinary sample variance ÷(n−1) at unit weights). Returns
// (nil, nil) when fewer than two pairs or the weight structure leaves no
// degrees of freedom.
func ciFromMoments(mean, sumWx, sumWx2, sumW, sumWsqrd float64, n int) (lower, upper *float64) {
	if n < 2 || sumW <= 0 {
		return nil, nil
	}
	d := sumW - sumWsqrd/sumW
	if d <= 0 {
		return nil, nil
	}
	variance := (sumWx2 - sumWx*sumWx/sumW) / d
	if variance < 0 {
		variance = 0 // floating-point guard
	}
	half := Z95 * math.Sqrt(variance) / math.Sqrt(float64(n))
	return ptr(mean - half), ptr(mean + half)
}

// MAECI returns the 95% CI for MAE (centred on MAE; s = std of |e|).
func (c *Continuous) MAECI() (lower, upper *float64) {
	m := c.MAE()
	if m == nil {
		return nil, nil
	}
	return ciFromMoments(*m, c.sumWeightAbs, c.sumWeightSq, c.sumWeight, c.sumWeightSqrd, c.n)
}

// RMSECI returns the 95% CI for RMSE (centred on RMSE; s = std of |e|, per
// methodology §7.4 — documented approximation).
func (c *Continuous) RMSECI() (lower, upper *float64) {
	m := c.RMSE()
	if m == nil {
		return nil, nil
	}
	return ciFromMoments(*m, c.sumWeightAbs, c.sumWeightSq, c.sumWeight, c.sumWeightSqrd, c.n)
}

// BiasCI returns the 95% CI for Bias (centred on Bias; s = std of the signed
// error e).
func (c *Continuous) BiasCI() (lower, upper *float64) {
	m := c.Bias()
	if m == nil {
		return nil, nil
	}
	return ciFromMoments(*m, c.sumWeightErr, c.sumWeightSq, c.sumWeight, c.sumWeightSqrd, c.n)
}

// ScoreCI returns the 95% CI for the Brier score (normal approximation on the
// mean of the per-pair squared errors; methodology §4.3 / UI contract).
func (b *Brier) ScoreCI() (lower, upper *float64) {
	s := b.Score()
	if s == nil {
		return nil, nil
	}
	return ciFromMoments(*s, b.sumWeightSq, b.sumWeightQuad, b.sumWeight, b.sumWeightSqrd, b.n)
}
