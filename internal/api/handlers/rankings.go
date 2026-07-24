package handlers

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/forecastiq/forecastiq/internal/analysis"
	analysisdomain "github.com/forecastiq/forecastiq/internal/analysis/domain"
	"github.com/forecastiq/forecastiq/internal/api/respond"
)

// Freshness thresholds by data class (conventions §2 examples; BR-FRESH).
// rankings: batch-computed, refreshed within ~60 s of the analysis run; a stale
// ranking simply reflects the last complete batch. observations: hourly source.
const (
	rankingsFreshMax = 75 * time.Minute
	rankingsStaleMax = 24 * time.Hour
	obsFreshMax      = 90 * time.Minute
	obsStaleMax      = 6 * time.Hour
)

// horizonProfiles is the closed set of accepted profile labels (methodology
// §6.3). Only `uniform` has stored rows in this release; the others resolve to
// an empty (freshness-unavailable) cohort — no misleading composite is invented.
var horizonProfiles = map[string]bool{
	analysisdomain.ProfileUniform: true,
	"short_term":                  true,
	"daily_planning":              true,
}

// providerRef is the compact provider identity + attribution on a ranking row.
type providerRef struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Slug        string         `json:"slug"`
	Attribution AttributionDTO `json:"attribution"`
}

// componentScoreDTO is one rounded composite-component breakdown entry.
type componentScoreDTO struct {
	Component  string   `json:"component"`
	Value      *float64 `json:"value"`
	Normalized *float64 `json:"normalized"`
	Weight     float64  `json:"weight"`
	Excluded   bool     `json:"excluded"`
}

// rankingRowDTO is one provider's ranking on the S-01 overview.
type rankingRowDTO struct {
	Rank                   int                 `json:"rank,omitempty"`
	Tied                   bool                `json:"tied,omitempty"`
	Provider               providerRef         `json:"provider"`
	CompositeScore         *float64            `json:"composite_score"`
	CILower                *float64            `json:"ci_lower,omitempty"`
	CIUpper                *float64            `json:"ci_upper,omitempty"`
	RankingStatus          string              `json:"ranking_status"`
	SampleCount            int                 `json:"sample_count"`
	Coverage               *float64            `json:"coverage,omitempty"`
	Reliability            *float64            `json:"reliability,omitempty"`
	CoveragePenaltyApplied bool                `json:"coverage_penalty_applied"`
	ComponentScores        []componentScoreDTO `json:"component_scores,omitempty"`
}

// observationContextDTO is the S-01 ground-truth context line (an observation
// record, never a weather product — NP-01).
type observationContextDTO struct {
	TemperatureC    *float64           `json:"temperature_c,omitempty"`
	PrecipitationMM *float64           `json:"precipitation_mm,omitempty"`
	ObservedAt      time.Time          `json:"observed_at"`
	Source          string             `json:"source"`
	ObservationType string             `json:"observation_type"`
	QualityFlag     string             `json:"quality_flag"`
	Freshness       *respond.Freshness `json:"freshness,omitempty"`
}

// evaluationPeriodDTO echoes the served evaluation window.
type evaluationPeriodDTO struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// rankingsDTO is the GET /rankings data block.
type rankingsDTO struct {
	LocationID         string                 `json:"location_id"`
	HorizonMinutes     int                    `json:"horizon_minutes"`
	HorizonProfile     string                 `json:"horizon_profile"`
	MinSampleCount     int                    `json:"min_sample_count"`
	EvaluationPeriod   *evaluationPeriodDTO   `json:"evaluation_period,omitempty"`
	Rankings           []rankingRowDTO        `json:"rankings"`
	ObservationContext *observationContextDTO `json:"observation_context,omitempty"`
}

// Rankings godoc
// @Summary      Provider rankings (S-01)
// @Description  Composite provider rankings for a location + horizon with full
// @Description  transparency (per-component breakdown, CIs, statuses, ties) and
// @Description  the latest observation context. Public; cached (ETag + LRU 60s).
// @Tags         analysis
// @Produce      json
// @Param        location_id      query string true  "location id (UUID)"
// @Param        horizon_minutes  query int    false "forecast horizon in minutes (default 1440)"
// @Param        horizon_profile  query string false "uniform|short_term|daily_planning (default uniform)"
// @Param        min_sample_count query int    false "ranked threshold echo (default 30)"
// @Param        period_days      query int    false "evaluation window hint (default 30)"
// @Success      200 {object} respond.Envelope
// @Failure      422 {object} respond.Problem
// @Router       /rankings [get]
func (h *Handlers) Rankings(c *gin.Context) {
	locationID, err := parseUUIDParam(c.Query("location_id"), "location_id")
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	horizon := analysis.DefaultRankingHorizon
	if v := c.Query("horizon_minutes"); v != "" {
		n, perr := strconv.Atoi(v)
		if perr != nil || n <= 0 {
			respond.Error(c, &fieldErr{"horizon_minutes", "must be a positive integer"}, respond.RequestID(c), c.Request.URL.Path)
			return
		}
		horizon = n
	}
	profile := c.DefaultQuery("horizon_profile", analysisdomain.ProfileUniform)
	if !horizonProfiles[profile] {
		respond.Error(c, &fieldErr{"horizon_profile", "must be one of uniform|short_term|daily_planning"}, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	minSample := analysis.DefaultMinSampleCount
	if v := c.Query("min_sample_count"); v != "" {
		n, perr := strconv.Atoi(v)
		if perr != nil || n < 0 {
			respond.Error(c, &fieldErr{"min_sample_count", "must be a non-negative integer"}, respond.RequestID(c), c.Request.URL.Path)
			return
		}
		minSample = n
	}
	if v := c.Query("period_days"); v != "" {
		if n, perr := strconv.Atoi(v); perr != nil || n <= 0 {
			respond.Error(c, &fieldErr{"period_days", "must be a positive integer"}, respond.RequestID(c), c.Request.URL.Path)
			return
		}
	}
	if c.Query("weights") != "" {
		respond.Error(c, &fieldErr{"weights", "custom weights are not supported in this release (uniform default only)"}, respond.RequestID(c), c.Request.URL.Path)
		return
	}

	res, err := h.Analysis.Rankings(c.Request.Context(), analysis.RankingsQuery{
		LocationID: locationID, HorizonMinutes: horizon, Profile: profile, MinSampleCount: minSample,
	})
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}

	dto := rankingsDTO{
		LocationID:     locationID.String(),
		HorizonMinutes: res.HorizonMinutes,
		HorizonProfile: res.HorizonProfile,
		MinSampleCount: res.MinSampleCount,
		Rankings:       make([]rankingRowDTO, 0, len(res.Rows)),
	}
	if res.HasRows {
		dto.EvaluationPeriod = &evaluationPeriodDTO{Start: res.PeriodStart.UTC(), End: res.PeriodEnd.UTC()}
	}
	seenAttr := map[string]bool{}
	var attribution []respond.Attribution
	for _, rr := range res.Rows {
		row := rr.Row
		dto.Rankings = append(dto.Rankings, rankingRowDTO{
			Rank: rr.Rank,
			Tied: rr.Tied,
			Provider: providerRef{
				ID: row.ProviderID.String(), Name: row.ProviderName, Slug: row.ProviderSlug,
				Attribution: AttributionDTO{Text: row.AttributionText, URL: row.AttributionURL},
			},
			CompositeScore:         respond.RoundScore(row.CompositeScore),
			CILower:                respond.RoundScore(row.CILower),
			CIUpper:                respond.RoundScore(row.CIUpper),
			RankingStatus:          row.Status,
			SampleCount:            row.SampleCount,
			Coverage:               respond.RoundScore(row.Coverage),
			Reliability:            respond.RoundScore(row.Reliability),
			CoveragePenaltyApplied: coveragePenaltyApplied(row.Coverage, row.Status),
			ComponentScores:        componentDTOs(row.ComponentScoresJSON),
		})
		if !seenAttr[row.AttributionText+row.AttributionURL] {
			seenAttr[row.AttributionText+row.AttributionURL] = true
			attribution = append(attribution, respond.Attribution{Provider: row.ProviderName, Text: row.AttributionText, URL: row.AttributionURL})
		}
	}

	now := time.Now().UTC()
	var provenance *respond.Provenance
	if res.Observation != nil {
		o := res.Observation
		dto.ObservationContext = &observationContextDTO{
			TemperatureC:    respond.RoundTemperature(o.TemperatureC),
			PrecipitationMM: respond.RoundRain(o.PrecipitationMM),
			ObservedAt:      o.ObservedAt.UTC(),
			Source:          o.Source,
			ObservationType: o.ObservationType,
			QualityFlag:     o.QualityFlag,
			Freshness:       respond.ComputeFreshness(&o.ObservedAt, now, obsFreshMax, obsStaleMax, ""),
		}
		provenance = &respond.Provenance{
			ObservationProvenanceMix: map[string]float64{o.ObservationType: 1.0},
			ObservationSources:       []string{o.Source},
		}
	}

	respond.OK(c, dto, respond.Options{
		RequestID:          respond.RequestID(c),
		MethodologyVersion: res.MethodologyVersion,
		WeightsVersion:     res.WeightsVersion,
		Freshness:          respond.ComputeFreshness(res.LastCalculatedAt, now, rankingsFreshMax, rankingsStaleMax, "no_rankings"),
		Provenance:         provenance,
		Attribution:        attribution,
	})
}

// RankingsMethodology godoc
// @Summary      Ranking methodology (S-06)
// @Description  The published statistical methodology: formulas, default
// @Description  weights, thresholds, coverage penalty, statuses, tie rule, and
// @Description  change history. Static config; long-cached.
// @Tags         analysis
// @Produce      json
// @Success      200 {object} respond.Envelope
// @Router       /rankings/methodology [get]
func (h *Handlers) RankingsMethodology(c *gin.Context) {
	doc := h.Analysis.Methodology()
	respond.OK(c, doc, respond.Options{
		RequestID:          respond.RequestID(c),
		MethodologyVersion: doc.MethodologyVersion,
		WeightsVersion:     doc.WeightsVersion,
	})
}

// coveragePenaltyApplied reports whether the §7.3 linear penalty was applied
// (coverage in [0.5, 0.8) on a scored row).
func coveragePenaltyApplied(coverage *float64, status string) bool {
	return status != analysisdomain.StatusUnranked && coverage != nil && *coverage < 0.8
}

// componentDTOs decodes and rounds the stored component_scores breakdown.
func componentDTOs(raw []byte) []componentScoreDTO {
	if len(raw) == 0 {
		return nil
	}
	var cs []analysisdomain.ComponentScore
	if err := json.Unmarshal(raw, &cs); err != nil {
		return nil
	}
	out := make([]componentScoreDTO, 0, len(cs))
	for _, c := range cs {
		out = append(out, componentScoreDTO{
			Component:  c.Component,
			Value:      roundComponentValue(c.Component, c.Value),
			Normalized: respond.RoundScore(c.Normalized),
			Weight:     roundWeight(c.Weight),
			Excluded:   c.Excluded,
		})
	}
	return out
}

// roundComponentValue rounds a raw composite-component value to its natural
// precision (the normalized ratio always uses score precision).
func roundComponentValue(component string, v *float64) *float64 {
	switch component {
	case "temp_mae", "temp_bias_abs":
		return respond.RoundTemperature(v)
	case "rain_mae_all":
		return respond.RoundRain(v)
	case "wind_mae":
		return respond.RoundWind(v)
	default: // precip_f1, coverage, reliability
		return respond.RoundScore(v)
	}
}

// roundWeight rounds a redistributed weight to 4 dp (score precision).
func roundWeight(w float64) float64 {
	if r := respond.RoundScore(&w); r != nil {
		return *r
	}
	return w
}
