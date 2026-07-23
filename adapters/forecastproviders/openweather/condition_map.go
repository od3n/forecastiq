package openweather

import "github.com/forecastiq/forecastiq/internal/collection/domain"

// mapCondition maps an OpenWeather condition code (weather[].id) to the
// canonical taxonomy v1 (domain §6). OpenWeather groups codes by hundreds
// (2xx thunderstorm, 3xx drizzle, 5xx rain, 6xx snow, 7xx atmosphere, 8xx
// clouds); the ranges below follow the published group semantics. Codes with
// no defensible canonical equivalent (e.g. 771 squall, 781 tornado) return
// (unknown, false) so the caller records the FC-15 unmapped metric — never
// guess a mapping. Versioned with the adapter.
func mapCondition(code int) (string, bool) {
	switch {
	case code >= 200 && code < 300:
		return domain.ConditionThunderstorm, true
	case code >= 300 && code < 400:
		return domain.ConditionDrizzle, true
	case code == 500 || code == 501 || (code >= 520 && code <= 522) || code == 531:
		return domain.ConditionRain, true
	case code == 502 || code == 503 || code == 504:
		return domain.ConditionHeavyRain, true
	case code == 511:
		return domain.ConditionSleet, true // freezing rain
	case code >= 600 && code <= 602:
		return domain.ConditionSnow, true
	case code >= 611 && code <= 616:
		return domain.ConditionSleet, true // rain-and-snow mix / sleet
	case code >= 620 && code <= 622:
		return domain.ConditionSnow, true // snow showers
	case code == 701 || code == 711 || code == 721 || code == 731 ||
		code == 741 || code == 751 || code == 761 || code == 762:
		return domain.ConditionFog, true // mist/smoke/haze/dust/fog/sand/ash
	case code == 800:
		return domain.ConditionClear, true
	case code == 801 || code == 802:
		return domain.ConditionPartlyCloudy, true
	case code == 803 || code == 804:
		return domain.ConditionCloudy, true
	default:
		return domain.ConditionUnknown, false
	}
}
