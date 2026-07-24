package respond

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func f(x float64) *float64 { return &x }

func TestRounding_ByKind(t *testing.T) {
	assert.Equal(t, 0.9568, *RoundScore(f(0.95681234)))
	assert.Equal(t, 31.24, *RoundTemperature(f(31.2361)))
	assert.Equal(t, 0.05, *RoundRain(f(0.049)))
	assert.Equal(t, 1013.25, *RoundPressure(f(1013.2549)))
	assert.Equal(t, 3.5, *RoundWind(f(3.451)))
}

func TestRounding_PreservesNil(t *testing.T) {
	assert.Nil(t, RoundScore(nil))
	assert.Nil(t, RoundMetric("temperature", "mae", nil))
}

func TestRoundMetric_VariablePrecision(t *testing.T) {
	// Continuous magnitudes carry their variable's precision.
	assert.Equal(t, 1.23, *RoundMetric("temperature", "mae", f(1.2345)))
	assert.Equal(t, 1.2, *RoundMetric("wind_speed", "mae", f(1.2345)))
	assert.Equal(t, 0.99, *RoundMetric("precipitation", "rain_mae_all", f(0.98765)))
	// Ratios/scores round to 4 dp regardless of variable.
	assert.Equal(t, 0.8421, *RoundMetric("precipitation", "f1", f(0.842105)))
	assert.Equal(t, 0.1925, *RoundMetric("precipitation", "brier", f(0.192512)))
	assert.Equal(t, 0.8, *RoundMetric("temperature", "coverage", f(0.8)))
}
