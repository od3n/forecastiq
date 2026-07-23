package eval

// Confusion is a weighted precipitation-occurrence confusion matrix
// (methodology §4.2). Pairs contribute fractionally by observation-quality
// weight (§6.4), so the counts are float. Metrics return nil for a zero
// denominator (§5: null, never 0/NaN). It is permutation-invariant.
//
//	                Observed rain   Observed dry
//	Forecast rain        TP              FP
//	Forecast dry         FN              TN
type Confusion struct {
	TP, FP, FN, TN float64
}

// Add classifies one eligible pair (rain/dry on each side) and adds its weight
// to the matching cell.
func (c *Confusion) Add(forecastRain, observedRain bool, weight float64) {
	switch {
	case forecastRain && observedRain:
		c.TP += weight
	case forecastRain && !observedRain:
		c.FP += weight
	case !forecastRain && observedRain:
		c.FN += weight
	default:
		c.TN += weight
	}
}

// N is the total weight across all cells.
func (c *Confusion) N() float64 { return c.TP + c.FP + c.FN + c.TN }

// Recall (POD) = TP/(TP+FN); nil when it never rained (TP+FN = 0).
func (c *Confusion) Recall() *float64 {
	d := c.TP + c.FN
	if d == 0 {
		return nil
	}
	return ptr(c.TP / d)
}

// Precision = TP/(TP+FP); nil when no rain was forecast (TP+FP = 0).
func (c *Confusion) Precision() *float64 {
	d := c.TP + c.FP
	if d == 0 {
		return nil
	}
	return ptr(c.TP / d)
}

// F1 = 2TP/(2TP+FP+FN); nil when the denominator is 0 (⇔ Precision and Recall
// both null).
func (c *Confusion) F1() *float64 {
	d := 2*c.TP + c.FP + c.FN
	if d == 0 {
		return nil
	}
	return ptr(2 * c.TP / d)
}

// FalseAlarmRate = FP/(FP+TN); nil when there were no dry hours (FP+TN = 0).
func (c *Confusion) FalseAlarmRate() *float64 {
	d := c.FP + c.TN
	if d == 0 {
		return nil
	}
	return ptr(c.FP / d)
}

// ThreatScore (CSI) = TP/(TP+FP+FN); nil when the denominator is 0.
func (c *Confusion) ThreatScore() *float64 {
	d := c.TP + c.FP + c.FN
	if d == 0 {
		return nil
	}
	return ptr(c.TP / d)
}

// OccurrenceAgreement = (TP+TN)/n; nil when empty. Published only with an
// imbalance warning and never ranked (methodology §4.2, §2).
func (c *Confusion) OccurrenceAgreement() *float64 {
	n := c.N()
	if n == 0 {
		return nil
	}
	return ptr((c.TP + c.TN) / n)
}
