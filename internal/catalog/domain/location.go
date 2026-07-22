package domain

import (
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
)

// DedupThresholdDegrees is the BR-LOC-01 near-duplicate proximity boundary.
// Two active locations strictly closer than this central angle are considered
// duplicates unless the caller supplies the override flag.
const DedupThresholdDegrees = 0.05

// Location is the catalog aggregate root for a forecast/observation site.
// Coordinates, country, timezone, and workspace are immutable after creation
// (a moved location is a new location — preserves historical data integrity).
type Location struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	Name        string
	Latitude    float64
	Longitude   float64
	CountryCode string
	Timezone    string
	Status      Status
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ValidateCreation checks the creation invariants and returns a populated
// ValidationError (or nil). Messages describe the constraint, never a stored
// value (security architecture §2.2).
func (l *Location) ValidateCreation() error {
	ve := &ValidationError{}
	if strings.TrimSpace(l.Name) == "" {
		ve.Add("name", "must not be empty")
	} else if len(l.Name) > 120 {
		ve.Add("name", "must be at most 120 characters")
	}
	if l.Latitude < -90 || l.Latitude > 90 {
		ve.Add("latitude", "must be between -90 and 90")
	}
	if l.Longitude < -180 || l.Longitude > 180 {
		ve.Add("longitude", "must be between -180 and 180")
	}
	if len(l.CountryCode) != 2 || strings.ToUpper(l.CountryCode) != l.CountryCode ||
		!isAlpha(l.CountryCode) {
		ve.Add("country_code", "must be a two-letter uppercase ISO 3166-1 alpha-2 code")
	}
	if strings.TrimSpace(l.Timezone) == "" {
		ve.Add("timezone", "must not be empty")
	} else if _, err := time.LoadLocation(l.Timezone); err != nil {
		ve.Add("timezone", "must be a valid IANA timezone identifier")
	}
	if !l.Status.Valid() {
		ve.Add("status", "must be one of active|disabled|archived")
	}
	return ve.ErrorOrNil()
}

func isAlpha(s string) bool {
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// IsNearDuplicate reports whether distanceDegrees falls inside the BR-LOC-01
// boundary (strictly less than the threshold; exactly 0.05° is permitted).
func IsNearDuplicate(distanceDegrees float64) bool {
	return distanceDegrees < DedupThresholdDegrees
}

// HaversineDegrees returns the great-circle central angle between two
// latitude/longitude points, in degrees. Used for the BR-LOC-01 dedup check.
func HaversineDegrees(lat1, lon1, lat2, lon2 float64) float64 {
	const deg2rad = math.Pi / 180
	phi1 := lat1 * deg2rad
	phi2 := lat2 * deg2rad
	dPhi := (lat2 - lat1) * deg2rad
	dLambda := (lon2 - lon1) * deg2rad

	a := math.Sin(dPhi/2)*math.Sin(dPhi/2) +
		math.Cos(phi1)*math.Cos(phi2)*math.Sin(dLambda/2)*math.Sin(dLambda/2)
	// Clamp against floating-point overshoot before the square root.
	if a > 1 {
		a = 1
	}
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return c * 180 / math.Pi
}
