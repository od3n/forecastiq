package openmeteo

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/forecastiq/forecastiq/internal/collection/domain"
	"github.com/forecastiq/forecastiq/internal/collection/ports"
	"github.com/forecastiq/forecastiq/internal/platform/ids"
)

// buildURL assembles the Open-Meteo Historical request for the [start, end]
// window. Base URLs are seeded configuration (not user input), so there is no
// SSRF surface. timezone=UTC pins UTC at the source (workflow §6).
func (a *Adapter) buildURL(req ports.ObservationRequest) string {
	base := req.BaseURL
	if base == "" {
		base = "https://historical-forecast-api.open-meteo.com"
	}
	q := url.Values{}
	q.Set("latitude", strconv.FormatFloat(req.Latitude, 'f', 6, 64))
	q.Set("longitude", strconv.FormatFloat(req.Longitude, 'f', 6, 64))
	q.Set("hourly", hourlyParams)
	q.Set("start_hour", req.WindowStart.UTC().Format(hourTimeLayout))
	q.Set("end_hour", req.WindowEnd.UTC().Format(hourTimeLayout))
	q.Set("timezone", "UTC")
	return base + forecastPath + "?" + q.Encode()
}

// ── Response schema (openmeteo-historical-v1) ─────────────────────────

type historicalResponse struct {
	Latitude  float64     `json:"latitude"`
	Longitude float64     `json:"longitude"`
	Timezone  string      `json:"timezone"`
	Hourly    *hourlyData `json:"hourly"`
}

type hourlyData struct {
	Time               []string   `json:"time"`
	Temperature2m      []*float64 `json:"temperature_2m"`
	RelativeHumidity2m []*float64 `json:"relative_humidity_2m"`
	WindSpeed10m       []*float64 `json:"wind_speed_10m"`
	WindDirection10m   []*float64 `json:"wind_direction_10m"`
	SurfacePressure    []*float64 `json:"surface_pressure"`
	Precipitation      []*float64 `json:"precipitation"`
	WeatherCode        []*int     `json:"weather_code"`
}

// decompose parses the raw payload, validates it against schema
// openmeteo-historical-v1, decomposes the hourly arrays into observation rows,
// normalizes (UTC, condition taxonomy, reanalysis provenance), and applies
// OC-04 range checks (violations → suspect, never dropped). It populates result
// in place.
func (a *Adapter) decompose(raw []byte, req ports.ObservationRequest, result *ports.ObservationResult) {
	var resp historicalResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		result.Outcome = ports.OutcomeFailed
		result.ErrorCode = "schema_drift"
		result.InvalidReasons = append(result.InvalidReasons, "payload is not valid JSON: "+err.Error())
		return
	}
	if resp.Hourly == nil || len(resp.Hourly.Time) == 0 {
		result.Outcome = ports.OutcomeFailed
		result.ErrorCode = "schema_drift"
		result.InvalidReasons = append(result.InvalidReasons, "required field hourly.time is missing or empty")
		return
	}

	h := resp.Hourly
	n := len(h.Time)
	result.RecordsReceived = n

	for i := 0; i < n; i++ {
		obs, reason := a.buildObservation(h, i, req)
		if obs == nil {
			result.InvalidCount++
			result.InvalidReasons = append(result.InvalidReasons, reason)
			continue
		}
		if reasons := obs.RangeReasons(); len(reasons) > 0 {
			// OC-04: keep the row, flag suspect (excluded from metrics downstream).
			obs.QualityFlag = domain.QualitySuspect
			result.SuspectCount++
			result.InvalidReasons = append(result.InvalidReasons,
				"row "+strconv.Itoa(i)+" suspect: "+strings.Join(reasons, ", "))
		}
		result.Observations = append(result.Observations, obs)
	}

	// Classify: structurally usable window with rows ⇒ success (suspects are
	// stored, not failures). >50% structurally invalid ⇒ schema drift.
	switch {
	case len(result.Observations) == 0:
		result.Outcome = ports.OutcomeFailed
		result.ErrorCode = "schema_drift"
	case result.InvalidCount*2 > n:
		result.Outcome = ports.OutcomeFailed
		result.ErrorCode = "schema_drift"
	default:
		result.Outcome = ports.OutcomeSuccess
	}
}

// buildObservation constructs and normalizes one observation row, or returns a
// rejection reason when the row cannot be built (missing/invalid observed_at,
// or an observed_at in the future — OC-04 observed_at ≤ now invariant is
// enforced against the requested window end).
func (a *Adapter) buildObservation(h *hourlyData, i int, req ports.ObservationRequest) (*domain.Observation, string) {
	observedAt, err := parseTime(h.Time[i])
	if err != nil {
		return nil, "row " + strconv.Itoa(i) + ": invalid observed_at " + strconv.Quote(h.Time[i])
	}
	if !req.WindowEnd.IsZero() && observedAt.After(req.WindowEnd.UTC()) {
		return nil, "row " + strconv.Itoa(i) + ": observed_at after window end (future)"
	}

	obs := &domain.Observation{
		ID:               ids.New(),
		LocationID:       req.LocationID,
		Source:           Source,
		ObservationType:  a.defaultType,
		ObservedAt:       observedAt,
		TemperatureC:     at(h.Temperature2m, i),
		HumidityPct:      at(h.RelativeHumidity2m, i),
		WindSpeedMS:      at(h.WindSpeed10m, i),
		WindDirectionDeg: at(h.WindDirection10m, i),
		PressureHPa:      at(h.SurfacePressure, i),
		PrecipitationMM:  at(h.Precipitation, i),
		QualityFlag:      domain.QualityValid,
	}

	if code := atInt(h.WeatherCode, i); code != nil {
		obs.ProviderConditionCode = strconv.Itoa(*code)
		obs.CanonicalConditionCode = mapCondition(*code)
	}
	return obs, ""
}

// at returns arr[i] or nil (out of range or JSON null).
func at(arr []*float64, i int) *float64 {
	if i < 0 || i >= len(arr) {
		return nil
	}
	return arr[i]
}

// atInt returns arr[i] or nil.
func atInt(arr []*int, i int) *int {
	if i < 0 || i >= len(arr) {
		return nil
	}
	return arr[i]
}

// parseTime parses an Open-Meteo hourly timestamp (UTC). The API returns
// "2006-01-02T15:04" when timezone=UTC; RFC3339 is accepted as a fallback.
func parseTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	t, err := time.Parse(hourTimeLayout, s)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}
