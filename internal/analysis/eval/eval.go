// Package eval is the pair-level evaluation kernel (WP-12): pure, deterministic
// functions that turn matched forecast–observation pairs into the continuous,
// categorical, and probabilistic metrics defined in docs/domain/03-metric-
// methodology.md §4. It carries no I/O and no persistence — the aggregation
// batch (WP-13) feeds eligible pairs into these accumulators to build metric
// rows. Nullable results use *float64 (nil = the methodology's `null`, never
// NaN/±Inf; §5).
package eval

// Observation provenance weights (methodology §6.4). A pair's contribution is
// scaled by the trustworthiness of its observation source. `corrected`
// observations retain their underlying observation_type, so weighting keys off
// observation_type alone; `suspect` never reaches the kernel (excluded by
// Eligible / §5).
const (
	WeightStation           = 1.0
	WeightInterpolated      = 0.8
	WeightReanalysis        = 0.8
	WeightProviderEstimated = 0.5

	// Observation types (match the observation_type enum).
	StationObservation = "station_observation"
	Interpolated       = "interpolated"
	Reanalysis         = "reanalysis"
	ProviderEstimated  = "provider_estimated"

	// QualitySuspect observations are excluded from every metric (§5).
	QualitySuspect = "suspect"

	// ObservedRainThresholdMM is the observed-rain cutoff (methodology §4.2:
	// precipitation_mm ≥ 0.1; trace below counts as dry).
	ObservedRainThresholdMM = 0.1
	// ForecastRainProbability is the forecast-rain probability cutoff
	// (methodology §4.2: precipitation_probability ≥ 0.5).
	ForecastRainProbability = 0.5
)

// ProvenanceWeight returns the observation-quality weight for an observation
// type (§6.4). Unknown types return 0 (defensive; the enum is constrained
// upstream), which safely drops the pair rather than misweighting it.
func ProvenanceWeight(observationType string) float64 {
	switch observationType {
	case StationObservation:
		return WeightStation
	case Interpolated:
		return WeightInterpolated
	case Reanalysis:
		return WeightReanalysis
	case ProviderEstimated:
		return WeightProviderEstimated
	default:
		return 0
	}
}

// Eligible reports whether a pair may contribute to a given variable's metric
// (methodology §3): both sides non-null and the observation not suspect. Time,
// location, and horizon eligibility are guaranteed upstream by the matching
// engine (WP-11).
func Eligible(forecast, observed *float64, qualityFlag string) bool {
	return forecast != nil && observed != nil && qualityFlag != QualitySuspect
}

// ForecastRain reports whether a forecast predicts rain for the hour
// (methodology §4.2): probability ≥ 0.5 OR amount > 0. A nil field simply does
// not trigger its clause.
func ForecastRain(precipProbability, precipAmount *float64) bool {
	if precipProbability != nil && *precipProbability >= ForecastRainProbability {
		return true
	}
	if precipAmount != nil && *precipAmount > 0 {
		return true
	}
	return false
}

// ObservedRain reports whether rain was observed for the hour (methodology
// §4.2): precipitation_mm ≥ 0.1. A nil observation is treated as dry.
func ObservedRain(precipMM *float64) bool {
	return precipMM != nil && *precipMM >= ObservedRainThresholdMM
}

// ptr returns a pointer to f (a non-null metric result).
func ptr(f float64) *float64 { return &f }
