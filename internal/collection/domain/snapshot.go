package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Physical validation ranges (domain §4.9; same ranges as observations OC-04).
const (
	MinTemperatureC = -90.0
	MaxTemperatureC = 60.0
	MinPressureHPa  = 870.0
	MaxPressureHPa  = 1084.0
	MaxWindSpeedMS  = 120.0
	MaxPrecipMM     = 500.0
)

// ForecastSnapshot is one predicted target period (immutable child entity).
// Weather fields are nullable per provider capability; probability is [0,1].
type ForecastSnapshot struct {
	ID                       uuid.UUID
	ForecastCollectionID     uuid.UUID
	ProviderID               uuid.UUID
	LocationID               uuid.UUID
	IssuedAt                 time.Time
	TargetTime               time.Time
	ForecastHorizonMinutes   int
	TemperatureC             *float64
	FeelsLikeTemperatureC    *float64
	PrecipitationProbability *float64
	PrecipitationAmountMM    *float64
	HumidityPct              *float64
	WindSpeedMS              *float64
	WindDirectionDeg         *float64
	PressureHPa              *float64
	CloudCoverPct            *float64
	ProviderConditionCode    string
	CanonicalConditionCode   string
	ConditionTaxonomyVersion string
	CreatedAt                time.Time
}

// Validate checks physical ranges and temporal sanity, returning the list of
// rejection reasons (empty means valid). Invalid rows are counted, not stored
// (domain §4.9).
func (s *ForecastSnapshot) Validate() []string {
	var reasons []string
	if !s.TargetTime.After(s.IssuedAt) {
		reasons = append(reasons, "target_time must be after issued_at")
	}
	if s.ForecastHorizonMinutes <= 0 {
		reasons = append(reasons, "forecast_horizon_minutes must be positive")
	}
	if v := s.TemperatureC; v != nil && (*v < MinTemperatureC || *v > MaxTemperatureC) {
		reasons = append(reasons, fmt.Sprintf("temperature_c %.2f out of range [%.0f,%.0f]", *v, MinTemperatureC, MaxTemperatureC))
	}
	if v := s.FeelsLikeTemperatureC; v != nil && (*v < MinTemperatureC || *v > MaxTemperatureC) {
		reasons = append(reasons, "feels_like_temperature_c out of range")
	}
	if v := s.PrecipitationProbability; v != nil && (*v < 0 || *v > 1) {
		reasons = append(reasons, "precipitation_probability must be within [0,1]")
	}
	if v := s.PrecipitationAmountMM; v != nil && (*v < 0 || *v > MaxPrecipMM) {
		reasons = append(reasons, "precipitation_amount_mm out of range")
	}
	if v := s.HumidityPct; v != nil && (*v < 0 || *v > 100) {
		reasons = append(reasons, "humidity_pct must be within [0,100]")
	}
	if v := s.WindSpeedMS; v != nil && (*v < 0 || *v > MaxWindSpeedMS) {
		reasons = append(reasons, "wind_speed_ms out of range")
	}
	if v := s.WindDirectionDeg; v != nil && (*v < 0 || *v > 360) {
		reasons = append(reasons, "wind_direction_deg must be within [0,360]")
	}
	if v := s.PressureHPa; v != nil && (*v < MinPressureHPa || *v > MaxPressureHPa) {
		reasons = append(reasons, "pressure_hpa out of range")
	}
	if v := s.CloudCoverPct; v != nil && (*v < 0 || *v > 100) {
		reasons = append(reasons, "cloud_cover_pct must be within [0,100]")
	}
	return reasons
}
