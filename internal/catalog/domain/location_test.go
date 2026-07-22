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

// TestIsNearDuplicate_BoundaryTable verifies the documented boundary semantics
// ("exactly 0.05° permitted") hold for coordinate pairs at exact 0.05°
// separation regardless of representation luck (DRB-WP04-002). Covers equator,
// mid-latitude, high latitude, and both meridional and zonal offsets.
func TestIsNearDuplicate_BoundaryTable(t *testing.T) {
	cases := []struct {
		name                   string
		lat1, lon1, lat2, lon2 float64
		expectDuplicate        bool
	}{
		// Meridional (latitude) offsets — exact 0.05° separation → permitted.
		{"equator meridional exact", 0, 0, 0.05, 0, false},
		{"mid-lat meridional exact", 1.4927, 103.7414, 1.5427, 103.7414, false},
		{"high-lat meridional exact", 60.0, 10.0, 60.05, 10.0, false},
		// The exact pair from DRB-WP04-002 that was incorrectly rejected live.
		{"DRB pair 20.001→20.051", 20.001, 60.0, 20.051, 60.0, false},
		// Zonal (longitude) offsets at equator — exact 0.05° separation → permitted.
		{"equator zonal exact", 0, 0, 0, 0.05, false},
		// 0.049° separation → rejected (inside boundary).
		{"equator meridional 0.049", 0, 0, 0.049, 0, true},
		{"mid-lat meridional 0.049", 1.4927, 103.7414, 1.5417, 103.7414, true},
		{"DRB pair 20.001→20.050", 20.001, 60.0, 20.050, 60.0, true},
		// 0.051° separation → permitted (outside boundary).
		{"equator meridional 0.051", 0, 0, 0.051, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dist := domain.HaversineDegrees(tc.lat1, tc.lon1, tc.lat2, tc.lon2)
			got := domain.IsNearDuplicate(dist)
			assert.Equal(t, tc.expectDuplicate, got,
				"dist=%.15f, expected duplicate=%v", dist, tc.expectDuplicate)
		})
	}
}
