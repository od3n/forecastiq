package openmeteo

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/forecastiq/forecastiq/internal/collection/domain"
	"github.com/forecastiq/forecastiq/internal/collection/ports"
	"github.com/forecastiq/forecastiq/internal/platform/ids"
)

// decompose parses the raw payload, validates it against schema openmeteo-v1,
// decomposes the hourly arrays into snapshot rows, normalizes (UTC, [0,1]
// probability, condition taxonomy), and validates each row's physical ranges.
// It populates result in place.
func (a *Adapter) decompose(raw []byte, req ports.ForecastRequest, result *ports.ForecastResult) {
	var resp forecastResponse
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
		snap, reason := a.buildSnapshot(h, i, req, result)
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
func (a *Adapter) buildSnapshot(h *hourlyData, i int, req ports.ForecastRequest, result *ports.ForecastResult) (*domain.ForecastSnapshot, string) {
	targetTime, err := parseTime(h.Time[i])
	if err != nil {
		return nil, "row " + strconv.Itoa(i) + ": invalid target_time " + strconv.Quote(h.Time[i])
	}
	horizon := int(targetTime.Sub(req.IssuedAt).Minutes())

	snap := &domain.ForecastSnapshot{
		ID:                       ids.New(),
		IssuedAt:                 req.IssuedAt,
		TargetTime:               targetTime,
		ForecastHorizonMinutes:   horizon,
		TemperatureC:             at(h.Temperature2m, i),
		FeelsLikeTemperatureC:    at(h.ApparentTemperature, i),
		PrecipitationProbability: pctToRatio(at(h.PrecipitationProbability, i)),
		PrecipitationAmountMM:    at(h.Precipitation, i),
		HumidityPct:              at(h.RelativeHumidity2m, i),
		WindSpeedMS:              at(h.WindSpeed10m, i),
		WindDirectionDeg:         at(h.WindDirection10m, i),
		PressureHPa:              at(h.SurfacePressure, i),
		CloudCoverPct:            at(h.CloudCover, i),
		ConditionTaxonomyVersion: domain.ConditionTaxonomyVersion,
	}

	if code := atInt(h.WeatherCode, i); code != nil {
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

// pctToRatio converts a 0–100 percentage to a [0,1] ratio (domain §5).
func pctToRatio(pct *float64) *float64 {
	if pct == nil {
		return nil
	}
	v := *pct / 100.0
	return &v
}

// parseTime parses an Open-Meteo hourly timestamp (UTC). The API returns
// "2006-01-02T15:04" when timezone=UTC; RFC3339 is accepted as a fallback.
func parseTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	t, err := time.Parse("2006-01-02T15:04", s)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}
