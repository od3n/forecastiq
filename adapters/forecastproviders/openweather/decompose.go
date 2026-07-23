package openweather

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/forecastiq/forecastiq/internal/collection/domain"
	"github.com/forecastiq/forecastiq/internal/collection/ports"
	"github.com/forecastiq/forecastiq/internal/platform/ids"
)

// ── Response schema (openweather-v1) ──────────────────────────────────
// One Call 3.0 with exclude=current,minutely,daily,alerts returns lat/lon,
// timezone metadata, and the 48-entry `hourly` array. Timestamps (`dt`) are
// unix seconds in UTC (BR-PROV-01: no local-time conversion needed).

type forecastResponse struct {
	Lat    float64       `json:"lat"`
	Lon    float64       `json:"lon"`
	Hourly []*hourlyItem `json:"hourly"`
}

type hourlyItem struct {
	Dt        int64           `json:"dt"`
	Temp      *float64        `json:"temp"`
	FeelsLike *float64        `json:"feels_like"`
	Pressure  *float64        `json:"pressure"`
	Humidity  *float64        `json:"humidity"`
	Clouds    *float64        `json:"clouds"`
	WindSpeed *float64        `json:"wind_speed"`
	WindDeg   *float64        `json:"wind_deg"`
	Pop       *float64        `json:"pop"` // probability of precipitation, already [0,1]
	Rain      *precipVolume   `json:"rain"`
	Snow      *precipVolume   `json:"snow"`
	Weather   []*weatherEntry `json:"weather"`
}

// precipVolume carries the last-hour accumulation (mm) under the "1h" key.
type precipVolume struct {
	OneHour *float64 `json:"1h"`
}

type weatherEntry struct {
	ID int `json:"id"`
}

// decompose parses the raw payload, validates it against schema openweather-v1,
// decomposes the hourly array into snapshot rows, normalizes (UTC, condition
// taxonomy), and validates each row's physical ranges. It populates result in
// place, mirroring the classification rules of the Open-Meteo adapter.
func (a *Adapter) decompose(raw []byte, req ports.ForecastRequest, result *ports.ForecastResult) {
	var resp forecastResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		result.Outcome = ports.OutcomeFailed
		result.ErrorCode = "schema_drift"
		result.InvalidReasons = append(result.InvalidReasons, "payload is not valid JSON: "+err.Error())
		return
	}
	if len(resp.Hourly) == 0 {
		result.Outcome = ports.OutcomeFailed
		result.ErrorCode = "schema_drift"
		result.InvalidReasons = append(result.InvalidReasons, "required field hourly is missing or empty")
		return
	}

	n := len(resp.Hourly)
	result.RecordsReceived = n

	for i, item := range resp.Hourly {
		snap, reason := a.buildSnapshot(item, i, req, result)
		if snap == nil {
			result.InvalidCount++
			result.InvalidReasons = append(result.InvalidReasons, reason)
			continue
		}
		if reasons := snap.Validate(); len(reasons) > 0 {
			result.InvalidCount++
			result.InvalidReasons = append(result.InvalidReasons,
				"row "+strconv.Itoa(i)+": "+strings.Join(reasons, ", "))
			continue
		}
		result.Snapshots = append(result.Snapshots, snap)
	}

	// Classify: >50% invalid → schema drift (failed); some invalid → partial.
	switch {
	case n == 0:
		result.Outcome = ports.OutcomeFailed
		result.ErrorCode = "schema_drift"
	case result.InvalidCount*2 > n:
		result.Outcome = ports.OutcomeFailed
		result.ErrorCode = "schema_drift"
	case result.InvalidCount > 0:
		result.Outcome = ports.OutcomePartial
	default:
		result.Outcome = ports.OutcomeSuccess
	}
}

// buildSnapshot constructs and normalizes one snapshot row, or returns a
// rejection reason when the row cannot be built (missing/invalid target time).
func (a *Adapter) buildSnapshot(item *hourlyItem, i int, req ports.ForecastRequest, result *ports.ForecastResult) (*domain.ForecastSnapshot, string) {
	if item == nil || item.Dt <= 0 {
		return nil, "row " + strconv.Itoa(i) + ": missing or invalid dt"
	}
	targetTime := time.Unix(item.Dt, 0).UTC()
	horizon := int(targetTime.Sub(req.IssuedAt).Minutes())

	snap := &domain.ForecastSnapshot{
		ID:                       ids.New(),
		IssuedAt:                 req.IssuedAt,
		TargetTime:               targetTime,
		ForecastHorizonMinutes:   horizon,
		TemperatureC:             item.Temp,
		FeelsLikeTemperatureC:    item.FeelsLike,
		PrecipitationProbability: item.Pop, // already a [0,1] ratio
		PrecipitationAmountMM:    precipMM(item),
		HumidityPct:              item.Humidity,
		WindSpeedMS:              item.WindSpeed,
		WindDirectionDeg:         item.WindDeg,
		PressureHPa:              item.Pressure,
		CloudCoverPct:            item.Clouds,
		ConditionTaxonomyVersion: domain.ConditionTaxonomyVersion,
	}

	if code := primaryConditionID(item.Weather); code != nil {
		snap.ProviderConditionCode = strconv.Itoa(*code)
		canonical, mapped := mapCondition(*code)
		snap.CanonicalConditionCode = canonical
		if !mapped {
			// FC-15: tally unmapped codes for the condition_unmapped metric.
			result.UnmappedConditions[snap.ProviderConditionCode]++
		}
	}
	return snap, ""
}

// precipMM returns the last-hour precipitation total (rain + snow, mm) or nil
// when the provider reported neither (a dry period, not a null field).
func precipMM(item *hourlyItem) *float64 {
	var total float64
	present := false
	if item.Rain != nil && item.Rain.OneHour != nil {
		total += *item.Rain.OneHour
		present = true
	}
	if item.Snow != nil && item.Snow.OneHour != nil {
		total += *item.Snow.OneHour
		present = true
	}
	if !present {
		return nil
	}
	return &total
}

// primaryConditionID returns the first weather entry's numeric code, or nil
// when the response carries no weather block for the period.
func primaryConditionID(entries []*weatherEntry) *int {
	for _, e := range entries {
		if e != nil {
			code := e.ID
			return &code
		}
	}
	return nil
}
