package domain

// Methodology publishing surface (GET /rankings/methodology, S-06). Fully
// static config derived from this package's constants + methodology doc §4–§7,
// so the endpoint can never drift from the engine that produces the rankings.

// Formula describes one published metric (methodology §4/§5).
type Formula struct {
	MetricType               string `json:"metric_type"`
	Formula                  string `json:"formula"`
	PlainLanguage            string `json:"plain_language"`
	Direction                string `json:"direction"` // lower_better | higher_better | direct
	ZeroDenominatorBehaviour string `json:"zero_denominator_behaviour"`
	Anchor                   string `json:"anchor"`
	Ranked                   bool   `json:"ranked"`
}

// WeightSpec is one default composite weight (methodology §6.3).
type WeightSpec struct {
	Component string  `json:"component"`
	Weight    float64 `json:"weight"`
	Direction string  `json:"direction"`
}

// StatusSpec is one ranking-status rule (methodology §7.2).
type StatusSpec struct {
	Status    string `json:"status"`
	Condition string `json:"condition"`
}

// CoveragePenalty describes the §7.3 penalty + outranking rule.
type CoveragePenalty struct {
	NoPenaltyAtOrAbove float64 `json:"no_penalty_at_or_above"`
	PenaltyRange       string  `json:"penalty_range"`
	Formula            string  `json:"formula"`
	UnrankedBelow      float64 `json:"unranked_below"`
	OutrankingRule     string  `json:"outranking_rule"`
}

// ChangeEntry is one methodology/weights change-history record (§7.5).
type ChangeEntry struct {
	MethodologyVersion string `json:"methodology_version"`
	WeightsVersion     string `json:"weights_version"`
	Date               string `json:"date"`
	Note               string `json:"note"`
}

// MethodologyDoc is the full /rankings/methodology payload.
type MethodologyDoc struct {
	MethodologyVersion string            `json:"methodology_version"`
	WeightsVersion     string            `json:"weights_version"`
	Formulas           []Formula         `json:"formulas"`
	DefaultWeights     []WeightSpec      `json:"default_weights"`
	Thresholds         map[string]int    `json:"thresholds"`
	CoveragePenalty    CoveragePenalty   `json:"coverage_penalty"`
	Statuses           []StatusSpec      `json:"statuses"`
	TieRule            string            `json:"tie_rule"`
	Rounding           map[string]string `json:"rounding"`
	HorizonProfiles    []string          `json:"horizon_profiles"`
	ChangeHistory      []ChangeEntry     `json:"change_history"`
	Docs               string            `json:"docs"`
}

// directionLabel maps the internal composite direction to its public label.
func directionLabel(d direction) string {
	switch d {
	case lowerBetter:
		return "lower_better"
	case higherBetter:
		return "higher_better"
	default:
		return "direct"
	}
}

// Methodology returns the published methodology document. It reads the same
// weights, thresholds, and penalty constants the ranking engine uses, so the
// S-06 page is always consistent with the composite it explains.
func Methodology() MethodologyDoc {
	weights := make([]WeightSpec, 0, len(Components))
	for _, c := range Components {
		weights = append(weights, WeightSpec{Component: c.Key, Weight: c.Weight, Direction: directionLabel(c.Direction)})
	}
	return MethodologyDoc{
		MethodologyVersion: MethodologyVersion,
		WeightsVersion:     WeightsVersion,
		Formulas: []Formula{
			{MetricType: MetricMAE, Formula: "(1/n)·Σ|f_i−o_i|", PlainLanguage: "Average magnitude of error, ignoring direction.", Direction: "lower_better", ZeroDenominatorBehaviour: "value null when n=0", Anchor: "§4.1", Ranked: true},
			{MetricType: MetricRMSE, Formula: "√((1/n)·Σ(f_i−o_i)²)", PlainLanguage: "Error magnitude penalizing large errors more heavily.", Direction: "lower_better", ZeroDenominatorBehaviour: "value null when n=0", Anchor: "§4.1"},
			{MetricType: MetricBias, Formula: "(1/n)·Σ(f_i−o_i)", PlainLanguage: "Systematic tendency: positive = forecasts run higher than reality.", Direction: "lower_better", ZeroDenominatorBehaviour: "value null when n=0", Anchor: "§4.1", Ranked: true},
			{MetricType: MetricRainMAEAll, Formula: "(1/n)·Σ|f_i−o_i| over all pairs", PlainLanguage: "Rainfall amount error including dry hours.", Direction: "lower_better", ZeroDenominatorBehaviour: "value null when n=0", Anchor: "§4.1", Ranked: true},
			{MetricType: MetricRainMAEWet, Formula: "(1/n)·Σ|f_i−o_i| where observed≥0.1mm", PlainLanguage: "Rainfall amount error, wet hours only (conditional skill).", Direction: "lower_better", ZeroDenominatorBehaviour: "value null when no wet hours", Anchor: "§4.1"},
			{MetricType: MetricRecall, Formula: "TP/(TP+FN)", PlainLanguage: "Of all hours it actually rained, how many were predicted.", Direction: "higher_better", ZeroDenominatorBehaviour: "null when TP+FN=0 (it never rained)", Anchor: "§4.2"},
			{MetricType: MetricPrecision, Formula: "TP/(TP+FP)", PlainLanguage: "Of all rain forecasts, how many were correct.", Direction: "higher_better", ZeroDenominatorBehaviour: "null when TP+FP=0", Anchor: "§4.2"},
			{MetricType: MetricF1, Formula: "2·TP/(2·TP+FP+FN)", PlainLanguage: "Balanced rain-occurrence score (harmonic mean of precision and recall).", Direction: "higher_better", ZeroDenominatorBehaviour: "null when 2·TP+FP+FN=0", Anchor: "§4.2", Ranked: true},
			{MetricType: MetricFAR, Formula: "FP/(FP+TN)", PlainLanguage: "Of all dry hours, how many were wrongly forecast as rain.", Direction: "lower_better", ZeroDenominatorBehaviour: "null when FP+TN=0", Anchor: "§4.2"},
			{MetricType: MetricThreatScore, Formula: "TP/(TP+FP+FN)", PlainLanguage: "Fraction of rain events correctly forecast (supplementary, not ranked).", Direction: "higher_better", ZeroDenominatorBehaviour: "null when denominator=0", Anchor: "§4.2"},
			{MetricType: MetricOccurrenceAgreement, Formula: "(TP+TN)/n", PlainLanguage: "Simple agreement — published with imbalance warning, never ranked.", Direction: "higher_better", ZeroDenominatorBehaviour: "null when n=0", Anchor: "§4.2"},
			{MetricType: MetricBrier, Formula: "(1/n)·Σ(p_i−a_i)²", PlainLanguage: "Calibration of rain probabilities. 0 = perfect, 1 = worst.", Direction: "lower_better", ZeroDenominatorBehaviour: "null when n=0", Anchor: "§4.3"},
			{MetricType: MetricCoverage, Formula: "delivered_snapshots/expected_snapshots", PlainLanguage: "How completely the provider delivered data.", Direction: "higher_better", ZeroDenominatorBehaviour: "null when no scheduled slots", Anchor: "§4.4", Ranked: true},
			{MetricType: MetricReliability, Formula: "successful_collections/scheduled_collections", PlainLanguage: "How reliably our collector retrieved the provider's data.", Direction: "higher_better", ZeroDenominatorBehaviour: "null when no scheduled slots", Anchor: "§4.4", Ranked: true},
		},
		DefaultWeights: weights,
		Thresholds:     map[string]int{"ranked": rankedMinPairs, "provisional": provisionalMinPairs},
		CoveragePenalty: CoveragePenalty{
			NoPenaltyAtOrAbove: coverageRankedMin,
			PenaltyRange:       "[0.5, 0.8)",
			Formula:            "final_score = score × (coverage / 0.8)",
			UnrankedBelow:      coverageFloor,
			OutrankingRule:     "A provider with coverage < 0.5 can never outrank one with coverage ≥ 0.8, regardless of score (BR-RANK-04).",
		},
		Statuses: []StatusSpec{
			{Status: StatusRanked, Condition: "≥ 30 pairs for all required variables and coverage ≥ 0.8 over ≥ 7 calendar days."},
			{Status: StatusProvisionally, Condition: "10–29 pairs for any required variable, coverage in [0.5, 0.8), or < 7 calendar days."},
			{Status: StatusUnranked, Condition: "< 10 pairs for any required variable, or coverage < 0.5 — no score ordering published."},
		},
		TieRule:         "Providers whose composite 95% confidence intervals overlap share a rank (tied group; BR-RANK-05).",
		Rounding:        map[string]string{"scores": "4 dp", "temperature_c": "2 dp", "rain_mm": "2 dp", "wind_ms": "1 dp", "pressure_hpa": "2 dp"},
		HorizonProfiles: []string{ProfileUniform},
		ChangeHistory: []ChangeEntry{
			{MethodologyVersion: MethodologyVersion, WeightsVersion: WeightsVersion, Date: "2026-08-01", Note: "Initial published methodology and default weights (w-2026.1)."},
		},
		Docs: "https://forecastiq.example/methodology/2026.1",
	}
}
