package openmeteo

import "github.com/forecastiq/forecastiq/internal/collection/domain"

// wmoConditionMap maps Open-Meteo WMO weather interpretation codes to the
// canonical taxonomy v1 (domain §6). It mirrors the forecast adapter's mapping
// (Open-Meteo Historical uses the same WMO codes) but is owned and versioned by
// this adapter independently — condition mapping is adapter-owned (authoring
// guide §4).
var wmoConditionMap = map[int]string{
	0:  domain.ConditionClear,        // Clear sky
	1:  domain.ConditionClear,        // Mainly clear
	2:  domain.ConditionPartlyCloudy, // Partly cloudy
	3:  domain.ConditionCloudy,       // Overcast
	45: domain.ConditionFog,          // Fog
	48: domain.ConditionFog,          // Depositing rime fog
	51: domain.ConditionDrizzle,      // Drizzle light
	53: domain.ConditionDrizzle,      // Drizzle moderate
	55: domain.ConditionDrizzle,      // Drizzle dense
	56: domain.ConditionSleet,        // Freezing drizzle light
	57: domain.ConditionSleet,        // Freezing drizzle dense
	61: domain.ConditionRain,         // Rain slight
	63: domain.ConditionRain,         // Rain moderate
	65: domain.ConditionHeavyRain,    // Rain heavy
	66: domain.ConditionSleet,        // Freezing rain light
	67: domain.ConditionSleet,        // Freezing rain heavy
	71: domain.ConditionSnow,         // Snow fall slight
	73: domain.ConditionSnow,         // Snow fall moderate
	75: domain.ConditionSnow,         // Snow fall heavy
	77: domain.ConditionSnow,         // Snow grains
	80: domain.ConditionRain,         // Rain showers slight
	81: domain.ConditionRain,         // Rain showers moderate
	82: domain.ConditionHeavyRain,    // Rain showers violent
	85: domain.ConditionSnow,         // Snow showers slight
	86: domain.ConditionSnow,         // Snow showers heavy
	95: domain.ConditionThunderstorm, // Thunderstorm
	96: domain.ConditionThunderstorm, // Thunderstorm with slight hail
	99: domain.ConditionThunderstorm, // Thunderstorm with heavy hail
}

// mapCondition maps a WMO code to its canonical condition, or unknown when the
// code is unrecognized (never guess a mapping).
func mapCondition(code int) string {
	if canonical, ok := wmoConditionMap[code]; ok {
		return canonical
	}
	return domain.ConditionUnknown
}
