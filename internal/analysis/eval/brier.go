package eval

// Brier accumulates the weighted Brier score for rain-probability forecasts
// (methodology §4.3):
//
//	BS = Σ wᵢ(pᵢ − aᵢ)² / Σ wᵢ    (aᵢ = 1 if rain observed, else 0)
//
// 0 = perfect, 1 = worst; supplementary in the MVP. With all weights 1 this is
// the unweighted (1/n)Σ(pᵢ−aᵢ)² (TV-4). Permutation-invariant.
type Brier struct {
	sumWeight   float64
	sumWeightSq float64 // Σ w(p−a)²
}

// Add includes one pair: forecast rain probability and whether rain was
// observed, at the given weight.
func (b *Brier) Add(probability float64, observedRain bool, weight float64) {
	a := 0.0
	if observedRain {
		a = 1.0
	}
	d := probability - a
	b.sumWeight += weight
	b.sumWeightSq += weight * d * d
}

// Score returns the weighted Brier score, or nil when empty (§5).
func (b *Brier) Score() *float64 {
	if b.sumWeight == 0 {
		return nil
	}
	return ptr(b.sumWeightSq / b.sumWeight)
}
