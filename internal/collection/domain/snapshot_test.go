package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/forecastiq/forecastiq/internal/collection/domain"
)

func fptr(v float64) *float64 { return &v }

func validSnapshot() *domain.ForecastSnapshot {
	return &domain.ForecastSnapshot{
		IssuedAt:                 time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC),
		TargetTime:               time.Date(2026, 7, 22, 11, 0, 0, 0, time.UTC),
		ForecastHorizonMinutes:   60,
		TemperatureC:             fptr(31.2),
		HumidityPct:              fptr(70),
		PrecipitationProbability: fptr(0.42),
	}
}

func TestSnapshot_Validate_Valid(t *testing.T) {
	assert.Empty(t, validSnapshot().Validate())
}

func TestSnapshot_Validate_OutOfRange(t *testing.T) {
	s := validSnapshot()
	s.TemperatureC = fptr(999)
	reasons := s.Validate()
	assert.NotEmpty(t, reasons)
}

func TestSnapshot_Validate_TargetNotAfterIssued(t *testing.T) {
	s := validSnapshot()
	s.TargetTime = s.IssuedAt // equal, not after
	assert.NotEmpty(t, s.Validate())
}

func TestSnapshot_Validate_BadHorizon(t *testing.T) {
	s := validSnapshot()
	s.ForecastHorizonMinutes = 0
	assert.NotEmpty(t, s.Validate())
}

func TestSnapshot_Validate_NullableFieldsAllowed(t *testing.T) {
	s := validSnapshot()
	s.TemperatureC = nil
	s.HumidityPct = nil
	s.PrecipitationProbability = nil
	assert.Empty(t, s.Validate())
}

func TestSnapshot_Validate_ProbabilityBounds(t *testing.T) {
	s := validSnapshot()
	s.PrecipitationProbability = fptr(1.5)
	assert.NotEmpty(t, s.Validate())
}
