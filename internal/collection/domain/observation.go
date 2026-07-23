package domain

import (
	"time"

	"github.com/google/uuid"
)

// ObservationType is the provenance of an observation value (ADR-003). Types
// drive quality weighting (methodology §6.4) and are always displayed. The MVP
// source (Open-Meteo Historical) is reanalysis-typed by default; station and
// interpolated arrive with future sources; provider_estimated is allowed only
// as a non-primary fallback (never truth).
type ObservationType string

const (
	ObservationStation           ObservationType = "station_observation"
	ObservationInterpolated      ObservationType = "interpolated"
	ObservationReanalysis        ObservationType = "reanalysis"
	ObservationProviderEstimated ObservationType = "provider_estimated"
)

// observationTypes is the closed set of valid provenance types.
var observationTypes = map[ObservationType]struct{}{
	ObservationStation: {}, ObservationInterpolated: {},
	ObservationReanalysis: {}, ObservationProviderEstimated: {},
}

// Valid reports whether t is a known observation type.
func (t ObservationType) Valid() bool {
	_, ok := observationTypes[t]
	return ok
}

// QualityFlag classifies an observation row (domain §2.7). Rows are created
// valid or suspect (range failure); a correction is a new row flagged corrected.
type QualityFlag string

const (
	QualityValid     QualityFlag = "valid"
	QualitySuspect   QualityFlag = "suspect"
	QualityCorrected QualityFlag = "corrected"
)

// Correction ε thresholds (workflow §4): a re-fetched value that differs by
// more than these is a genuine source correction rather than float noise.
// Values within ε are treated as equal (dedup, no correction).
const (
	CorrectionEpsilonTemperatureC = 0.1
	CorrectionEpsilonPrecipMM     = 0.05
	CorrectionEpsilonHumidityPct  = 1.0
	CorrectionEpsilonWindSpeedMS  = 0.1
	CorrectionEpsilonWindDirDeg   = 1.0
	CorrectionEpsilonPressureHPa  = 0.5
)

// Observation is one measured weather record for a location-hour (immutable
// root, domain §2.7). Weather fields are nullable per source capability; the
// only permitted mutation is setting SupersededObservationID on the old row
// when a correction replaces it. Physical ranges match forecast snapshots
// (OC-04 == domain §4.9 ranges).
type Observation struct {
	ID                      uuid.UUID
	LocationID              uuid.UUID
	Source                  string // e.g. openmeteo_historical
	ObservationType         ObservationType
	ObservedAt              time.Time
	TemperatureC            *float64
	HumidityPct             *float64
	WindSpeedMS             *float64
	WindDirectionDeg        *float64
	PressureHPa             *float64
	PrecipitationMM         *float64
	ProviderConditionCode   string
	CanonicalConditionCode  string
	QualityFlag             QualityFlag
	SupersededObservationID *uuid.UUID
	CreatedAt               time.Time
}

// RangeReasons returns the OC-04 physical-range violations for this
// observation (empty means all present values are in range). A non-empty
// result means the row must be flagged suspect (workflow §5) — stored, not
// rejected, and excluded from metrics.
func (o *Observation) RangeReasons() []string {
	var reasons []string
	if v := o.TemperatureC; v != nil && (*v < MinTemperatureC || *v > MaxTemperatureC) {
		reasons = append(reasons, "temperature_c out of range")
	}
	if v := o.HumidityPct; v != nil && (*v < 0 || *v > 100) {
		reasons = append(reasons, "humidity_pct out of range")
	}
	if v := o.WindSpeedMS; v != nil && (*v < 0 || *v > MaxWindSpeedMS) {
		reasons = append(reasons, "wind_speed_ms out of range")
	}
	if v := o.WindDirectionDeg; v != nil && (*v < 0 || *v > 360) {
		reasons = append(reasons, "wind_direction_deg out of range")
	}
	if v := o.PressureHPa; v != nil && (*v < MinPressureHPa || *v > MaxPressureHPa) {
		reasons = append(reasons, "pressure_hpa out of range")
	}
	if v := o.PrecipitationMM; v != nil && (*v < 0 || *v > MaxPrecipMM) {
		reasons = append(reasons, "precipitation_mm out of range")
	}
	return reasons
}

// DiffersFrom reports whether o carries a genuine correction relative to prev
// (an already-collected row for the same source/location/hour): any comparable
// weather value differs beyond its ε threshold (workflow §4). A value that is
// present on one side and absent on the other counts as a difference. Values
// all within ε ⇒ equal ⇒ deduplicated, no correction.
func (o *Observation) DiffersFrom(prev *Observation) bool {
	return floatDiffers(o.TemperatureC, prev.TemperatureC, CorrectionEpsilonTemperatureC) ||
		floatDiffers(o.PrecipitationMM, prev.PrecipitationMM, CorrectionEpsilonPrecipMM) ||
		floatDiffers(o.HumidityPct, prev.HumidityPct, CorrectionEpsilonHumidityPct) ||
		floatDiffers(o.WindSpeedMS, prev.WindSpeedMS, CorrectionEpsilonWindSpeedMS) ||
		floatDiffers(o.WindDirectionDeg, prev.WindDirectionDeg, CorrectionEpsilonWindDirDeg) ||
		floatDiffers(o.PressureHPa, prev.PressureHPa, CorrectionEpsilonPressureHPa)
}

// floatDiffers reports whether two optional values differ meaningfully: a
// presence mismatch always differs; two present values differ when |a-b| > eps.
func floatDiffers(a, b *float64, eps float64) bool {
	if a == nil && b == nil {
		return false
	}
	if a == nil || b == nil {
		return true
	}
	d := *a - *b
	if d < 0 {
		d = -d
	}
	return d > eps
}
