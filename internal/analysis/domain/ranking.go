package domain

import (
	"sort"

	"github.com/google/uuid"
)

// WeightsVersion stamps every ranking row (methodology §6.3 / §7.5).
const WeightsVersion = "w-2026.1"

// Ranking statuses (methodology §7.2; ranking_status enum).
const (
	StatusRanked        = "ranked"
	StatusProvisionally = "provisionally_ranked"
	StatusUnranked      = "unranked"
)

// Horizon profiles (methodology §6.3).
const (
	ProfileUniform = "uniform"
)

// epsilon guards ratio-to-best normalization when the best value is 0
// (methodology §6.2).
const epsilon = 1e-9

// Sample / coverage thresholds (methodology §7.1–§7.3; BR-RANK-02/04/09).
const (
	rankedMinPairs      = 30
	provisionalMinPairs = 10
	coverageRankedMin   = 0.8
	coverageFloor       = 0.5
	minCalendarDays     = 7
)

// direction of a composite component.
type direction int

const (
	lowerBetter direction = iota
	higherBetter
	directValue
)

// Component is one composite input (methodology §6.1/§6.3).
type Component struct {
	Key       string
	Weight    float64
	Direction direction
}

// Components is the default composite specification (weights w-2026.1; §6.3).
var Components = []Component{
	{Key: "temp_mae", Weight: 0.30, Direction: lowerBetter},
	{Key: "precip_f1", Weight: 0.25, Direction: higherBetter},
	{Key: "rain_mae_all", Weight: 0.15, Direction: lowerBetter},
	{Key: "wind_mae", Weight: 0.15, Direction: lowerBetter},
	{Key: "temp_bias_abs", Weight: 0.05, Direction: lowerBetter},
	{Key: "coverage", Weight: 0.05, Direction: directValue},
	{Key: "reliability", Weight: 0.05, Direction: directValue},
}

// ProviderInput is one provider's cohort inputs for a ranking cell.
type ProviderInput struct {
	ProviderID uuid.UUID
	// Values holds the raw component value per Component.Key (nil = null metric,
	// excluded with weight redistribution; temp_bias_abs is already |bias|).
	Values map[string]*float64
	// CIHalf holds the ±half-width of each component's CI (nil if none), used for
	// first-order composite CI propagation.
	CIHalf map[string]*float64
	// SampleCount per required variable (temperature, precipitation) and coverage
	// drive status; CalendarDays gates BR-RANK-09.
	TempSampleCount   int
	PrecipSampleCount int
	CalendarDays      int
	Coverage          *float64
	Reliability       *float64
}

// ComponentScore is the per-component breakdown stored in component_scores.
type ComponentScore struct {
	Component  string   `json:"component"`
	Value      *float64 `json:"value"`
	Normalized *float64 `json:"normalized"`
	Weight     float64  `json:"weight"`
	Excluded   bool     `json:"excluded"`
}

// ProviderRanking is one computed ranking row (before persistence ids/period).
type ProviderRanking struct {
	ProviderID      uuid.UUID
	CompositeScore  *float64 // nil ⇔ unranked
	CILower         *float64
	CIUpper         *float64
	Status          string
	SampleCount     int
	Coverage        *float64
	Reliability     *float64
	ComponentScores []ComponentScore
}

// RankCohort computes the composite ranking for every provider in a cohort at
// one (location, horizon, period), per methodology §6–§7: ratio-to-best
// normalization with null-component redistribution and an ε guard, weighted
// sum, coverage penalty, status, and a first-order composite CI. Deterministic
// and permutation-invariant over the input order.
func RankCohort(cohort []ProviderInput) []ProviderRanking {
	// A component is active only if every provider has a non-null value for it
	// (methodology §6.2). Excluded components' weight is redistributed
	// proportionally to the active ones.
	active := map[string]bool{}
	activeWeight := 0.0
	for _, c := range Components {
		allPresent := len(cohort) > 0
		for _, p := range cohort {
			if v, ok := p.Values[c.Key]; !ok || v == nil {
				allPresent = false
				break
			}
		}
		active[c.Key] = allPresent
		if allPresent {
			activeWeight += c.Weight
		}
	}

	// Per active component, find the cohort best for ratio normalization.
	best := map[string]float64{}
	shift := map[string]float64{} // ε shift applied when best == 0 (lower-better)
	for _, c := range Components {
		if !active[c.Key] || c.Direction == directValue {
			continue
		}
		vals := make([]float64, 0, len(cohort))
		for _, p := range cohort {
			vals = append(vals, *p.Values[c.Key])
		}
		switch c.Direction {
		case lowerBetter:
			m := minOf(vals)
			if m == 0 {
				shift[c.Key] = epsilon
				m += epsilon
			}
			best[c.Key] = m
		case higherBetter:
			best[c.Key] = maxOf(vals)
		}
	}

	out := make([]ProviderRanking, 0, len(cohort))
	for _, p := range cohort {
		r := ProviderRanking{
			ProviderID:  p.ProviderID,
			Coverage:    p.Coverage,
			Reliability: p.Reliability,
			SampleCount: minInt(p.TempSampleCount, p.PrecipSampleCount),
		}
		composite := 0.0
		ciHalf := 0.0
		for _, c := range Components {
			cs := ComponentScore{Component: c.Key, Value: p.Values[c.Key], Weight: c.Weight, Excluded: !active[c.Key]}
			if !active[c.Key] {
				r.ComponentScores = append(r.ComponentScores, cs)
				continue
			}
			w := c.Weight / activeWeight // redistributed
			cs.Weight = w
			norm := normalize(c, *p.Values[c.Key], best[c.Key], shift[c.Key])
			cs.Normalized = f64(norm)
			composite += w * norm
			// First-order CI propagation: relative error of the raw value carries
			// into the normalized component (direct components have no CI here).
			if h := p.CIHalf[c.Key]; h != nil && c.Direction != directValue && *p.Values[c.Key] != 0 {
				ciHalf += w * norm * (*h / *p.Values[c.Key])
			}
			r.ComponentScores = append(r.ComponentScores, cs)
		}

		r.Status = status(p)
		if r.Status == StatusUnranked {
			out = append(out, r) // composite/CI nil
			continue
		}
		// Coverage penalty (methodology §7.3): [0.5, 0.8) → ×(coverage/0.8).
		if p.Coverage != nil && *p.Coverage < coverageRankedMin {
			penalty := *p.Coverage / coverageRankedMin
			composite *= penalty
			ciHalf *= penalty
		}
		composite = clamp01(composite)
		r.CompositeScore = f64(composite)
		if ciHalf > 0 {
			r.CILower = f64(clamp01(composite - ciHalf))
			r.CIUpper = f64(clamp01(composite + ciHalf))
		}
		out = append(out, r)
	}
	return out
}

// normalize applies the §6.2 rule for one component value.
func normalize(c Component, value, best, shift float64) float64 {
	switch c.Direction {
	case lowerBetter:
		return best / (value + shift)
	case higherBetter:
		if best == 0 {
			return 0
		}
		return value / best
	default: // directValue (coverage/reliability already in [0,1])
		return value
	}
}

// status applies the §7.2 thresholds (BR-RANK-02/04/09).
func status(p ProviderInput) string {
	cov := 0.0
	if p.Coverage != nil {
		cov = *p.Coverage
	}
	if p.TempSampleCount < provisionalMinPairs || p.PrecipSampleCount < provisionalMinPairs || cov < coverageFloor {
		return StatusUnranked
	}
	if p.TempSampleCount < rankedMinPairs || p.PrecipSampleCount < rankedMinPairs ||
		cov < coverageRankedMin || p.CalendarDays < minCalendarDays {
		return StatusProvisionally
	}
	return StatusRanked
}

// RankOrder returns provider indexes ordered by descending composite among
// ranked/provisional rows, grouping providers whose CIs overlap into the same
// rank number (methodology §7.4; BR-RANK-05). Unranked rows are excluded.
// Returned as groups of provider indexes (into rankings) sharing a rank.
func RankOrder(rankings []ProviderRanking) [][]int {
	idx := make([]int, 0, len(rankings))
	for i, r := range rankings {
		if r.CompositeScore != nil {
			idx = append(idx, i)
		}
	}
	sort.SliceStable(idx, func(a, b int) bool {
		return *rankings[idx[a]].CompositeScore > *rankings[idx[b]].CompositeScore
	})
	var groups [][]int
	for _, i := range idx {
		if n := len(groups); n > 0 {
			// Tie with the current group's leader if CIs overlap.
			lead := groups[n-1][0]
			if ciOverlap(rankings[lead], rankings[i]) {
				groups[n-1] = append(groups[n-1], i)
				continue
			}
		}
		groups = append(groups, []int{i})
	}
	return groups
}

func ciOverlap(a, b ProviderRanking) bool {
	if a.CILower == nil || a.CIUpper == nil || b.CILower == nil || b.CIUpper == nil {
		return false
	}
	return *a.CILower <= *b.CIUpper && *b.CILower <= *a.CIUpper
}

func minOf(v []float64) float64 {
	m := v[0]
	for _, x := range v[1:] {
		if x < m {
			m = x
		}
	}
	return m
}

func maxOf(v []float64) float64 {
	m := v[0]
	for _, x := range v[1:] {
		if x > m {
			m = x
		}
	}
	return m
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

func f64(x float64) *float64 { return &x }
