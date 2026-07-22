package domain

// ConditionTaxonomyVersion is the MVP canonical weather-condition taxonomy
// version (domain §6). Recorded on every snapshot at ingest; adding codes is
// a new version (no retroactive rewrite).
const ConditionTaxonomyVersion = "1"

// Canonical condition codes (taxonomy v1).
const (
	ConditionClear        = "clear"
	ConditionPartlyCloudy = "partly_cloudy"
	ConditionCloudy       = "cloudy"
	ConditionFog          = "fog"
	ConditionDrizzle      = "drizzle"
	ConditionRain         = "rain"
	ConditionHeavyRain    = "heavy_rain"
	ConditionThunderstorm = "thunderstorm"
	ConditionSnow         = "snow"
	ConditionSleet        = "sleet"
	ConditionUnknown      = "unknown" // unmapped / unrecognized provider code
)

// canonicalCodes is the closed set of valid canonical codes.
var canonicalCodes = map[string]struct{}{
	ConditionClear: {}, ConditionPartlyCloudy: {}, ConditionCloudy: {},
	ConditionFog: {}, ConditionDrizzle: {}, ConditionRain: {},
	ConditionHeavyRain: {}, ConditionThunderstorm: {}, ConditionSnow: {},
	ConditionSleet: {}, ConditionUnknown: {},
}

// IsValidCanonical reports whether code is a known canonical condition.
func IsValidCanonical(code string) bool {
	_, ok := canonicalCodes[code]
	return ok
}
