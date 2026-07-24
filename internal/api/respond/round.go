package respond

import "math"

// Rounding decimal places by value kind (conventions §7, methodology §5).
// Storage retains full precision; rounding is a presentation-layer concern
// applied by the API when assembling DTOs.
const (
	dpScore       = 4 // ratios and composite scores
	dpTemperature = 2 // °C
	dpRain        = 2 // mm
	dpPressure    = 2 // hPa
	dpWind        = 1 // m/s
)

// round rounds x to n decimal places (half away from zero).
func round(x float64, n int) float64 {
	p := math.Pow(10, float64(n))
	return math.Round(x*p) / p
}

// roundPtr rounds a nullable value, preserving nil (null cells are never
// coerced to 0 — conventions §7 / CSV rule).
func roundPtr(x *float64, n int) *float64 {
	if x == nil {
		return nil
	}
	r := round(*x, n)
	return &r
}

// RoundScore rounds a ratio/composite score to 4 dp.
func RoundScore(x *float64) *float64 { return roundPtr(x, dpScore) }

// RoundTemperature rounds a temperature (°C) to 2 dp.
func RoundTemperature(x *float64) *float64 { return roundPtr(x, dpTemperature) }

// RoundRain rounds a precipitation amount (mm) to 2 dp.
func RoundRain(x *float64) *float64 { return roundPtr(x, dpRain) }

// RoundPressure rounds a pressure (hPa) to 2 dp.
func RoundPressure(x *float64) *float64 { return roundPtr(x, dpPressure) }

// RoundWind rounds a wind speed (m/s) to 1 dp.
func RoundWind(x *float64) *float64 { return roundPtr(x, dpWind) }

// RoundMetric rounds a metric value to the decimal places appropriate to the
// (variable, metric_type) pair (methodology §5): scores/ratios 4 dp; the
// temperature/wind/pressure error magnitudes carry their variable's precision;
// rain magnitudes 2 dp. Unknown combinations round to 4 dp (score precision).
func RoundMetric(variable, metricType string, x *float64) *float64 {
	if x == nil {
		return nil
	}
	switch metricType {
	case "mae", "rmse", "bias", "rain_mae_all", "rain_mae_wet":
		switch variable {
		case "temperature":
			return RoundTemperature(x)
		case "wind_speed":
			return RoundWind(x)
		case "pressure":
			return RoundPressure(x)
		case "humidity":
			return RoundScore(x)
		default: // precipitation magnitudes (mm)
			return RoundRain(x)
		}
	default: // recall/precision/f1/far/threat_score/occurrence_agreement/brier/coverage/reliability
		return RoundScore(x)
	}
}
