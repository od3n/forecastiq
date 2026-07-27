package handlers

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/analysis"
	analysisdomain "github.com/forecastiq/forecastiq/internal/analysis/domain"
	"github.com/forecastiq/forecastiq/internal/api/respond"
	"github.com/forecastiq/forecastiq/internal/catalog"
)

// comparisonVariables is the closed set of FvA variables (methodology §4).
var comparisonVariables = map[string]bool{
	"temperature": true, "wind_speed": true, "humidity": true,
	"pressure": true, "precipitation": true,
}

// ── DTOs ────────────────────────────────────────────────────────────────

type fvaLocationDTO struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
}

type fvaPointDTO struct {
	TargetTime     time.Time `json:"target_time"`
	Value          *float64  `json:"value"`
	IssuedAt       time.Time `json:"issued_at"`
	HorizonMinutes int       `json:"horizon_minutes"`
}

type fvaSeriesDTO struct {
	Provider providerRef   `json:"provider"`
	IssuedAt time.Time     `json:"issued_at"`
	Points   []fvaPointDTO `json:"points"`
}

type fvaObservationDTO struct {
	ObservedAt      time.Time `json:"observed_at"`
	Value           *float64  `json:"value"`
	Source          string    `json:"source"`
	ObservationType string    `json:"observation_type"`
	QualityFlag     string    `json:"quality_flag"`
	ConditionCode   *string   `json:"condition_code,omitempty"`
}

type fvaDayMetricDTO struct {
	ProviderID         string   `json:"provider_id"`
	MAE                *float64 `json:"mae"`
	RMSE               *float64 `json:"rmse"`
	Bias               *float64 `json:"bias"`
	SampleCount        int      `json:"sample_count"`
	MethodologyVersion string   `json:"methodology_version"`
}

type fvaDTO struct {
	Location              fvaLocationDTO      `json:"location"`
	Date                  string              `json:"date"`
	Variable              string              `json:"variable"`
	HorizonMinutes        int                 `json:"horizon_minutes"`
	Series                []fvaSeriesDTO      `json:"series"`
	Observations          []fvaObservationDTO `json:"observations"`
	DayMetrics            []fvaDayMetricDTO   `json:"day_metrics"`
	ErrorBandMAE          *float64            `json:"error_band_mae,omitempty"`
	ObservationsAvailable bool                `json:"observations_available"`
}

// ForecastComparison godoc
// @Summary      Forecast vs. Actual (S-05)
// @Description  Bounded public payload for one location + day + variable: per-
// @Description  provider forecast lines at the DR-02-selected issuance, the
// @Description  day's observations (gaps absent), and in-memory day metrics.
// @Tags         analysis
// @Produce      json
// @Param        location_id     query string true  "location id (UUID)"
// @Param        date            query string true  "ISO date (interpreted in the location timezone)"
// @Param        variable        query string true  "temperature|wind_speed|humidity|pressure|precipitation"
// @Param        horizon_minutes query int    true  "forecast horizon (issuance selection, DR-02)"
// @Param        providers       query string false "CSV of provider UUIDs (default: all active)"
// @Success      200 {object} respond.Envelope
// @Failure      404 {object} respond.Problem
// @Failure      422 {object} respond.Problem
// @Router       /forecast-comparison [get]
func (h *Handlers) ForecastComparison(c *gin.Context) {
	locationID, err := parseUUIDParam(c.Query("location_id"), "location_id")
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	variable := c.Query("variable")
	if !comparisonVariables[variable] {
		respond.Error(c, &fieldErr{"variable", "must be one of temperature|wind_speed|humidity|pressure|precipitation"}, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	horizonRaw := c.Query("horizon_minutes")
	if horizonRaw == "" {
		respond.Error(c, &fieldErr{"horizon_minutes", "is required"}, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	horizon, herr := strconv.Atoi(horizonRaw)
	if herr != nil || horizon <= 0 {
		respond.Error(c, &fieldErr{"horizon_minutes", "must be a positive integer"}, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	dateRaw := c.Query("date")
	if dateRaw == "" {
		respond.Error(c, &fieldErr{"date", "is required (ISO date)"}, respond.RequestID(c), c.Request.URL.Path)
		return
	}

	// Location resolves first (404 on unknown); its timezone anchors the day.
	loc, err := h.Locations.GetLocation(c.Request.Context(), locationID)
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	tz, tzErr := time.LoadLocation(loc.Timezone)
	if tzErr != nil {
		// A stored location always has a valid IANA zone (validated on create);
		// fall back to UTC defensively rather than 500.
		tz = time.UTC
	}
	day, derr := time.ParseInLocation("2006-01-02", dateRaw, tz)
	if derr != nil {
		respond.Error(c, &fieldErr{"date", "must be an ISO date (YYYY-MM-DD)"}, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	from := day.UTC()
	to := day.AddDate(0, 0, 1).UTC()

	// Providers: explicit CSV or all active.
	providerIDs, refs, perr := h.resolveComparisonProviders(c, locationID)
	if perr {
		return
	}

	res, err := h.Analysis.ForecastComparison(c.Request.Context(), analysis.ComparisonQuery{
		LocationID: locationID, ProviderIDs: providerIDs, Variable: variable,
		HorizonMinutes: horizon, From: from, To: to,
	})
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}

	dto := fvaDTO{
		Location:              fvaLocationDTO{ID: loc.ID.String(), Name: loc.Name, Timezone: loc.Timezone},
		Date:                  dateRaw,
		Variable:              variable,
		HorizonMinutes:        horizon,
		Series:                make([]fvaSeriesDTO, 0, len(res.Series)),
		Observations:          make([]fvaObservationDTO, 0, len(res.Observations)),
		DayMetrics:            make([]fvaDayMetricDTO, 0, len(res.DayMetrics)),
		ErrorBandMAE:          respond.RoundMetric(variable, "mae", res.ErrorBandMAE),
		ObservationsAvailable: res.ObservationsAvailable,
	}
	present := map[uuid.UUID]bool{}
	var attribution []respond.Attribution
	seenAttr := map[string]bool{}
	for _, sr := range res.Series {
		present[sr.ProviderID] = true
		ref := refs[sr.ProviderID]
		points := make([]fvaPointDTO, 0, len(sr.Points))
		for _, p := range sr.Points {
			v := p.Value
			points = append(points, fvaPointDTO{
				TargetTime: p.TargetTime.UTC(), Value: roundValue(variable, &v),
				IssuedAt: p.IssuedAt.UTC(), HorizonMinutes: p.HorizonMinutes,
			})
		}
		dto.Series = append(dto.Series, fvaSeriesDTO{Provider: ref, IssuedAt: sr.IssuedAt.UTC(), Points: points})
		if ref.Attribution.Text != "" && !seenAttr[ref.Attribution.Text] {
			seenAttr[ref.Attribution.Text] = true
			attribution = append(attribution, respond.Attribution{Provider: ref.Name, Text: ref.Attribution.Text, URL: ref.Attribution.URL})
		}
	}
	for _, o := range res.Observations {
		v := o.Value
		dto.Observations = append(dto.Observations, fvaObservationDTO{
			ObservedAt: o.ObservedAt.UTC(), Value: roundValue(variable, &v),
			Source: o.Source, ObservationType: o.ObservationType, QualityFlag: o.QualityFlag,
			ConditionCode: o.ConditionCode,
		})
	}
	for _, m := range res.DayMetrics {
		dto.DayMetrics = append(dto.DayMetrics, fvaDayMetricDTO{
			ProviderID:  m.ProviderID.String(),
			MAE:         respond.RoundMetric(variable, "mae", m.MAE),
			RMSE:        respond.RoundMetric(variable, "rmse", m.RMSE),
			Bias:        respond.RoundMetric(variable, "bias", m.Bias),
			SampleCount: m.SampleCount, MethodologyVersion: analysisdomain.MethodologyVersion,
		})
	}

	var provenance *respond.Provenance
	if len(res.ProvenanceMix) > 0 {
		provenance = &respond.Provenance{ObservationProvenanceMix: res.ProvenanceMix, QualityWeightingApplied: boolPtr(true)}
	}
	var warnings []respond.Warning
	for _, id := range providerIDs {
		if !present[id] {
			warnings = append(warnings, respond.Warning{
				ProviderID: id.String(), Code: warnProviderUnavailable,
				Message: "No forecast for this provider on the requested day/horizon.",
			})
		}
	}
	if len(present) == 0 {
		warnings = nil // all-absent is not a partial result (§4.2 rule 6)
	}

	respond.OK(c, dto, respond.Options{
		RequestID:          respond.RequestID(c),
		Timezone:           loc.Timezone,
		MethodologyVersion: analysisdomain.MethodologyVersion,
		Freshness:          respond.ComputeFreshness(res.LatestObservedAt, time.Now().UTC(), obsFreshMax, obsStaleMax, "observations_unavailable"),
		Provenance:         provenance,
		Attribution:        attribution,
		Warnings:           warnings,
	})
}

// resolveComparisonProviders returns the requested provider ids (explicit CSV,
// or all active) plus an id→identity map. On a malformed CSV it writes a 422
// and returns aborted=true.
func (h *Handlers) resolveComparisonProviders(c *gin.Context, _ uuid.UUID) (ids []uuid.UUID, refs map[uuid.UUID]providerRef, aborted bool) {
	all, err := h.Providers.ListProviders(c.Request.Context())
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return nil, nil, true
	}
	refs = make(map[uuid.UUID]providerRef, len(all))
	active := make(map[uuid.UUID]bool, len(all))
	for _, p := range all {
		refs[p.ID] = providerRef{
			ID: p.ID.String(), Name: p.Name, Slug: p.Slug,
			Attribution: AttributionDTO{Text: p.AttributionText, URL: p.AttributionURL},
		}
		if p.Status == catalog.StatusActive {
			active[p.ID] = true
		}
	}
	raw := strings.TrimSpace(c.Query("providers"))
	if raw == "" {
		for _, p := range all {
			if active[p.ID] {
				ids = append(ids, p.ID)
			}
		}
		return ids, refs, false
	}
	for _, tok := range strings.Split(raw, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		id, perr := uuid.Parse(tok)
		if perr != nil {
			respond.Error(c, &fieldErr{"providers", "must be a comma-separated list of UUIDs"}, respond.RequestID(c), c.Request.URL.Path)
			return nil, nil, true
		}
		ids = append(ids, id)
	}
	return ids, refs, false
}

// roundValue rounds a raw variable value to its natural presentation precision
// (conventions §7): temperature/pressure/rain 2 dp, wind 1 dp, humidity 4 dp.
func roundValue(variable string, v *float64) *float64 {
	switch variable {
	case "temperature":
		return respond.RoundTemperature(v)
	case "wind_speed":
		return respond.RoundWind(v)
	case "pressure":
		return respond.RoundPressure(v)
	case "precipitation":
		return respond.RoundRain(v)
	default: // humidity (%)
		return respond.RoundScore(v)
	}
}
