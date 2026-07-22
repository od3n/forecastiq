package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/forecastiq/forecastiq/internal/catalog/domain"
)

func validLocation() *domain.Location {
	return &domain.Location{
		Name: "Johor Bahru", Latitude: 1.4927, Longitude: 103.7414,
		CountryCode: "MY", Timezone: "Asia/Kuala_Lumpur", Status: domain.StatusActive,
	}
}

func TestLocation_ValidateCreation_Valid(t *testing.T) {
	assert.NoError(t, validLocation().ValidateCreation())
}

func TestLocation_ValidateCreation_Invalid(t *testing.T) {
	cases := map[string]func(*domain.Location){
		"empty name":     func(l *domain.Location) { l.Name = "" },
		"lat too high":   func(l *domain.Location) { l.Latitude = 91 },
		"lon too low":    func(l *domain.Location) { l.Longitude = -181 },
		"bad country":    func(l *domain.Location) { l.CountryCode = "my" },
		"bad timezone":   func(l *domain.Location) { l.Timezone = "Not/AZone" },
		"empty timezone": func(l *domain.Location) { l.Timezone = "" },
		"bad status":     func(l *domain.Location) { l.Status = "bogus" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			l := validLocation()
			mutate(l)
			err := l.ValidateCreation()
			assert.Error(t, err, name)
			var ve *domain.ValidationError
			assert.ErrorAs(t, err, &ve)
		})
	}
}

func TestHaversineDegrees(t *testing.T) {
	// Same point → 0.
	assert.InDelta(t, 0.0, domain.HaversineDegrees(1.4927, 103.7414, 1.4927, 103.7414), 1e-9)
	// 0.05° of latitude is a 0.05° central angle.
	assert.InDelta(t, 0.05, domain.HaversineDegrees(0, 0, 0.05, 0), 1e-6)
	// Symmetry.
	d1 := domain.HaversineDegrees(1.0, 2.0, 3.0, 4.0)
	d2 := domain.HaversineDegrees(3.0, 4.0, 1.0, 2.0)
	assert.InDelta(t, d1, d2, 1e-9)
}

func TestIsNearDuplicate_Boundary(t *testing.T) {
	// Strictly less than 0.05° is a duplicate; exactly 0.05° is permitted.
	assert.True(t, domain.IsNearDuplicate(0.049))
	assert.False(t, domain.IsNearDuplicate(domain.DedupThresholdDegrees))
	assert.False(t, domain.IsNearDuplicate(0.06))
}
