package domain_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/forecastiq/forecastiq/internal/collection/domain"
)

func f64(v float64) *float64 { return &v }

func TestObservationType_Valid(t *testing.T) {
	assert.True(t, domain.ObservationReanalysis.Valid())
	assert.True(t, domain.ObservationStation.Valid())
	assert.False(t, domain.ObservationType("guess").Valid())
}

func TestObservation_RangeReasons(t *testing.T) {
	valid := &domain.Observation{
		TemperatureC: f64(31.2), HumidityPct: f64(70), WindSpeedMS: f64(3.5),
		WindDirectionDeg: f64(180), PressureHPa: f64(1013), PrecipitationMM: f64(0.2),
	}
	assert.Empty(t, valid.RangeReasons(), "in-range observation has no violations")

	// nil fields are not violations (nullable per source capability).
	assert.Empty(t, (&domain.Observation{}).RangeReasons())

	hot := &domain.Observation{TemperatureC: f64(999)}
	assert.Len(t, hot.RangeReasons(), 1)

	bad := &domain.Observation{
		TemperatureC: f64(-200), HumidityPct: f64(150), WindSpeedMS: f64(500),
		WindDirectionDeg: f64(400), PressureHPa: f64(10), PrecipitationMM: f64(-1),
	}
	assert.Len(t, bad.RangeReasons(), 6, "every out-of-range field is reported")
}

// TestObservation_DiffersFrom exercises the correction ε logic (workflow §4):
// within-ε changes are float noise (dedup); beyond-ε changes are corrections;
// a presence mismatch always differs.
func TestObservation_DiffersFrom(t *testing.T) {
	base := &domain.Observation{TemperatureC: f64(31.2), PrecipitationMM: f64(0.2)}

	assert.False(t, (&domain.Observation{TemperatureC: f64(31.2), PrecipitationMM: f64(0.2)}).DiffersFrom(base),
		"identical values do not differ")
	assert.False(t, (&domain.Observation{TemperatureC: f64(31.25), PrecipitationMM: f64(0.2)}).DiffersFrom(base),
		"temperature within ε (0.05 ≤ 0.1) is float noise")
	assert.True(t, (&domain.Observation{TemperatureC: f64(31.6), PrecipitationMM: f64(0.2)}).DiffersFrom(base),
		"temperature beyond ε (0.4 > 0.1) is a correction")
	assert.True(t, (&domain.Observation{TemperatureC: f64(31.2), PrecipitationMM: f64(0.3)}).DiffersFrom(base),
		"precipitation beyond ε (0.1 > 0.05) is a correction")
	assert.True(t, (&domain.Observation{TemperatureC: f64(31.2)}).DiffersFrom(base),
		"presence mismatch (precip now absent) differs")
	assert.False(t, (&domain.Observation{}).DiffersFrom(&domain.Observation{}),
		"two empty observations do not differ")
}

func TestObservation_MutationInvariant(t *testing.T) {
	// The only permitted mutation is setting SupersededObservationID (domain §2.7).
	newID := uuid.New()
	old := &domain.Observation{TemperatureC: f64(31.2), QualityFlag: domain.QualityValid}
	old.SupersededObservationID = &newID
	assert.Equal(t, newID, *old.SupersededObservationID)
	assert.Equal(t, domain.QualityValid, old.QualityFlag, "flag untouched by supersession")
	assert.Equal(t, 31.2, *old.TemperatureC, "weather values untouched by supersession")
}
