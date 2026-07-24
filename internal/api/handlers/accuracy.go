package handlers

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/analysis"
	analysisdomain "github.com/forecastiq/forecastiq/internal/analysis/domain"
	analysisports "github.com/forecastiq/forecastiq/internal/analysis/ports"
	"github.com/forecastiq/forecastiq/internal/api/respond"
)

// Freshness + bound constants for the accuracy endpoints.
const (
	summaryFreshMax  = 90 * time.Minute
	summaryStaleMax  = 24 * time.Hour
	trendsFreshMax   = 26 * time.Hour // a daily bucket closes at the day boundary
	trendsStaleMax   = 8 * 24 * time.Hour
	trendsMaxDays    = 365 // BR: /accuracy range bound
	trendsDefaultDay = 30
	trendsMaxBuckets = 2000 // response-size guard (< 80 KB; caching §5)
)

// aggregations is the closed set of trend bucket spans (methodology §7.1).
var aggregations = map[string]bool{"daily": true, "weekly": true, "monthly": true}

// ── DTOs shared by the accuracy endpoints ───────────────────────────────

type metricDTO struct {
	Variable    string   `json:"variable"`
	MetricType  string   `json:"metric_type"`
	Value       *float64 `json:"value"`
	CILower     *float64 `json:"ci_lower,omitempty"`
	CIUpper     *float64 `json:"ci_upper,omitempty"`
	SampleCount int      `json:"sample_count"`
}

type collectionWindowDTO struct {
	FirstSnapshotAt *time.Time `json:"first_snapshot_at,omitempty"`
	LastSnapshotAt  *time.Time `json:"last_snapshot_at,omitempty"`
	Coverage        *float64   `json:"coverage,omitempty"`
	Reliability     *float64   `json:"reliability,omitempty"`
}

func windowDTO(w analysisports.CollectionWindow) collectionWindowDTO {
	return collectionWindowDTO{
		FirstSnapshotAt: utcPtr(w.FirstSnapshotAt), LastSnapshotAt: utcPtr(w.LastSnapshotAt),
		Coverage: respond.RoundScore(w.Coverage), Reliability: respond.RoundScore(w.Reliability),
	}
}

type locationSummaryProviderDTO struct {
	Provider         providerRef         `json:"provider"`
	RankingStatus    string              `json:"ranking_status,omitempty"`
	Metrics          []metricDTO         `json:"metrics"`
	CollectionWindow collectionWindowDTO `json:"collection_window"`
}

type providerSummaryCellDTO struct {
	LocationID       string              `json:"location_id"`
	LocationName     string              `json:"location_name"`
	HorizonMinutes   int                 `json:"horizon_minutes"`
	CompositeScore   *float64            `json:"composite_score"`
	RankingStatus    string              `json:"ranking_status"`
	SampleCount      int                 `json:"sample_count"`
	Coverage         *float64            `json:"coverage,omitempty"`
	CollectionWindow collectionWindowDTO `json:"collection_window"`
}

// AccuracySummary godoc
// @Summary      Accuracy summary (S-02 location / S-03 provider)
// @Description  Location mode (location_id): all providers' metric grid,
// @Description  ranking status, and collection window. Provider mode
// @Description  (provider_id): all ranking cells for a provider. Exactly one of
// @Description  location_id or provider_id is required. Public; cached.
// @Tags         analysis
// @Produce      json
// @Param        location_id     query string false "location id (UUID) — location mode"
// @Param        provider_id     query string false "provider id (UUID) — provider mode"
// @Param        horizon_minutes query int    false "horizon (location mode; default 1440)"
// @Param        period_days     query int    false "evaluation window hint (default 30)"
// @Success      200 {object} respond.Envelope
// @Failure      422 {object} respond.Problem
// @Router       /accuracy/summary [get]
func (h *Handlers) AccuracySummary(c *gin.Context) {
	loc := c.Query("location_id")
	prov := c.Query("provider_id")
	if (loc == "") == (prov == "") {
		respond.Error(c, &fieldErr{"location_id", "exactly one of location_id or provider_id is required"}, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	if prov != "" {
		h.accuracySummaryByProvider(c, prov)
		return
	}
	h.accuracySummaryByLocation(c, loc)
}

func (h *Handlers) accuracySummaryByLocation(c *gin.Context, loc string) {
	locationID, err := parseUUIDParam(loc, "location_id")
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	horizon, ok := horizonParam(c)
	if !ok {
		return
	}
	res, err := h.Analysis.LocationSummary(c.Request.Context(), locationID, horizon)
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	refs, err := h.providerRefs(c.Request.Context())
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}

	providers := make([]locationSummaryProviderDTO, 0, len(res.Providers))
	var attribution []respond.Attribution
	seen := map[string]bool{}
	for _, p := range res.Providers {
		metrics := make([]metricDTO, 0, len(p.Metrics))
		for _, m := range p.Metrics {
			metrics = append(metrics, metricDTO{
				Variable: m.Variable, MetricType: m.MetricType,
				Value:       respond.RoundMetric(m.Variable, m.MetricType, m.Value),
				CILower:     respond.RoundMetric(m.Variable, m.MetricType, m.CILower),
				CIUpper:     respond.RoundMetric(m.Variable, m.MetricType, m.CIUpper),
				SampleCount: m.SampleCount,
			})
		}
		ref := refs[p.ProviderID]
		providers = append(providers, locationSummaryProviderDTO{
			Provider: ref, RankingStatus: p.RankingStatus, Metrics: metrics,
			CollectionWindow: windowDTO(p.Window),
		})
		if ref.Attribution.Text != "" && !seen[ref.Attribution.Text] {
			seen[ref.Attribution.Text] = true
			attribution = append(attribution, respond.Attribution{Provider: ref.Name, Text: ref.Attribution.Text, URL: ref.Attribution.URL})
		}
	}

	respond.OK(c, gin.H{
		"mode": "location", "location_id": locationID.String(),
		"horizon_minutes": res.HorizonMinutes, "providers": providers,
	}, respond.Options{
		RequestID:          respond.RequestID(c),
		MethodologyVersion: analysisdomain.MethodologyVersion,
		WeightsVersion:     analysisdomain.WeightsVersion,
		Freshness:          respond.ComputeFreshness(res.LastSnapshotAt, time.Now().UTC(), summaryFreshMax, summaryStaleMax, "no_data"),
		Provenance:         &respond.Provenance{QualityWeightingApplied: boolPtr(true)},
		Attribution:        attribution,
	})
}

func (h *Handlers) accuracySummaryByProvider(c *gin.Context, prov string) {
	providerID, err := parseUUIDParam(prov, "provider_id")
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	res, err := h.Analysis.ProviderSummary(c.Request.Context(), providerID)
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	provider, err := h.Providers.GetProvider(c.Request.Context(), providerID)
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	ref := providerRef{
		ID: provider.ID.String(), Name: provider.Name, Slug: provider.Slug,
		Attribution: AttributionDTO{Text: provider.AttributionText, URL: provider.AttributionURL},
	}

	var lastSnapshot *time.Time
	cells := make([]providerSummaryCellDTO, 0, len(res.Cells))
	for _, cell := range res.Cells {
		w := res.Windows[cell.LocationID]
		w.Coverage, w.Reliability = cell.Coverage, cell.Reliability
		if w.LastSnapshotAt != nil && (lastSnapshot == nil || w.LastSnapshotAt.After(*lastSnapshot)) {
			lastSnapshot = w.LastSnapshotAt
		}
		cells = append(cells, providerSummaryCellDTO{
			LocationID: cell.LocationID.String(), LocationName: cell.LocationName,
			HorizonMinutes: cell.HorizonMinutes, CompositeScore: respond.RoundScore(cell.CompositeScore),
			RankingStatus: cell.RankingStatus, SampleCount: cell.SampleCount,
			Coverage: respond.RoundScore(cell.Coverage), CollectionWindow: windowDTO(w),
		})
	}

	respond.OK(c, gin.H{"mode": "provider", "provider": ref, "cells": cells}, respond.Options{
		RequestID:          respond.RequestID(c),
		MethodologyVersion: analysisdomain.MethodologyVersion,
		WeightsVersion:     analysisdomain.WeightsVersion,
		Freshness:          respond.ComputeFreshness(lastSnapshot, time.Now().UTC(), summaryFreshMax, summaryStaleMax, "no_data"),
		Provenance:         &respond.Provenance{QualityWeightingApplied: boolPtr(true)},
		Attribution:        []respond.Attribution{{Provider: ref.Name, Text: ref.Attribution.Text, URL: ref.Attribution.URL}},
	})
}

// ── /accuracy trends ────────────────────────────────────────────────────

type trendBucketDTO struct {
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	Value       *float64  `json:"value"`
	CILower     *float64  `json:"ci_lower,omitempty"`
	CIUpper     *float64  `json:"ci_upper,omitempty"`
	SampleCount int       `json:"sample_count"`
}

type trendSeriesDTO struct {
	ProviderID string           `json:"provider_id"`
	Buckets    []trendBucketDTO `json:"buckets"`
}

// AccuracyTrends godoc
// @Summary      Accuracy trends (S-04)
// @Description  Time-bucketed metric series for a location + horizon + variable
// @Description  + metric_type over a bounded period (≤ 365 days). Every bucket
// @Description  carries its sample_count (hollow-point support). Public; cached.
// @Tags         analysis
// @Produce      json
// @Param        location_id   query string true  "location id (UUID)"
// @Param        variable      query string true  "metric variable"
// @Param        metric_type   query string true  "metric type"
// @Param        provider_id   query string false "restrict to one provider"
// @Param        horizon_minutes query int  false "horizon (default 1440)"
// @Param        aggregation   query string false "daily|weekly|monthly (default daily)"
// @Param        period_start  query string false "ISO date/time (default now−30d)"
// @Param        period_end    query string false "ISO date/time (default now)"
// @Param        tz            query string false "IANA bucketing zone (echoed)"
// @Param        limit         query int    false "max buckets (default/cap 2000)"
// @Success      200 {object} respond.Envelope
// @Failure      422 {object} respond.Problem
// @Router       /accuracy [get]
func (h *Handlers) AccuracyTrends(c *gin.Context) {
	locationID, err := parseUUIDParam(c.Query("location_id"), "location_id")
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	variable := c.Query("variable")
	metricType := c.Query("metric_type")
	if variable == "" || metricType == "" {
		respond.Error(c, &fieldErr{"variable", "variable and metric_type are required"}, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	horizon, ok := horizonParam(c)
	if !ok {
		return
	}
	aggregation := c.DefaultQuery("aggregation", "daily")
	if !aggregations[aggregation] {
		respond.Error(c, &fieldErr{"aggregation", "must be one of daily|weekly|monthly"}, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	tz := c.DefaultQuery("tz", "UTC")
	if _, terr := time.LoadLocation(tz); terr != nil {
		respond.Error(c, &fieldErr{"tz", "must be a valid IANA timezone"}, respond.RequestID(c), c.Request.URL.Path)
		return
	}

	now := time.Now().UTC()
	to := now
	from := now.AddDate(0, 0, -trendsDefaultDay)
	if v := c.Query("period_end"); v != "" {
		t, perr := parseDateOrTime(v)
		if perr != nil {
			respond.Error(c, &fieldErr{"period_end", "must be an ISO date or date-time"}, respond.RequestID(c), c.Request.URL.Path)
			return
		}
		to = t
	}
	if v := c.Query("period_start"); v != "" {
		t, perr := parseDateOrTime(v)
		if perr != nil {
			respond.Error(c, &fieldErr{"period_start", "must be an ISO date or date-time"}, respond.RequestID(c), c.Request.URL.Path)
			return
		}
		from = t
	}
	if !from.Before(to) {
		respond.Error(c, &fieldErr{"period_start", "must be before period_end"}, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	if to.Sub(from) > trendsMaxDays*24*time.Hour {
		respond.Error(c, &fieldErr{"period_end", "range must not exceed 365 days"}, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	limit := trendsMaxBuckets
	if v := c.Query("limit"); v != "" {
		n, perr := strconv.Atoi(v)
		if perr != nil || n <= 0 {
			respond.Error(c, &fieldErr{"limit", "must be a positive integer"}, respond.RequestID(c), c.Request.URL.Path)
			return
		}
		if n < limit {
			limit = n
		}
	}

	f := analysisports.TrendFilter{
		LocationID: locationID, HorizonMinutes: horizon, Variable: variable, MetricType: metricType,
		From: from, To: to, Aggregation: aggregation, Limit: limit,
	}
	pid, okp := queryUUID(c, "provider_id")
	if !okp {
		return
	}
	f.ProviderID = pid // nil when absent (all providers)

	res, err := h.Analysis.Trends(c.Request.Context(), f)
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	series := make([]trendSeriesDTO, 0, len(res.Series))
	for _, sr := range res.Series {
		buckets := make([]trendBucketDTO, 0, len(sr.Buckets))
		for _, b := range sr.Buckets {
			buckets = append(buckets, trendBucketDTO{
				PeriodStart: b.PeriodStart.UTC(), PeriodEnd: b.PeriodEnd.UTC(),
				Value:       respond.RoundMetric(variable, metricType, b.Value),
				CILower:     respond.RoundMetric(variable, metricType, b.CILower),
				CIUpper:     respond.RoundMetric(variable, metricType, b.CIUpper),
				SampleCount: b.SampleCount,
			})
		}
		series = append(series, trendSeriesDTO{ProviderID: sr.ProviderID.String(), Buckets: buckets})
	}

	respond.OK(c, gin.H{
		"location_id": locationID.String(), "horizon_minutes": horizon,
		"variable": variable, "metric_type": metricType, "aggregation": aggregation, "tz": tz,
		"period_start": from.UTC(), "period_end": to.UTC(), "series": series,
	}, respond.Options{
		RequestID:          respond.RequestID(c),
		Timezone:           tz,
		MethodologyVersion: analysisdomain.MethodologyVersion,
		Freshness:          respond.ComputeFreshness(res.LastPeriodEnd, now, trendsFreshMax, trendsStaleMax, "no_data"),
	})
}

// ── helpers ─────────────────────────────────────────────────────────────

// horizonParam parses the optional horizon_minutes (default 1440), writing a
// 422 and returning ok=false on an invalid value.
func horizonParam(c *gin.Context) (int, bool) {
	horizon := analysis.DefaultRankingHorizon
	if v := c.Query("horizon_minutes"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			respond.Error(c, &fieldErr{"horizon_minutes", "must be a positive integer"}, respond.RequestID(c), c.Request.URL.Path)
			return 0, false
		}
		horizon = n
	}
	return horizon, true
}

// providerRefs builds an id→identity map from the catalog for DTO assembly.
func (h *Handlers) providerRefs(ctx context.Context) (map[uuid.UUID]providerRef, error) {
	list, err := h.Providers.ListProviders(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]providerRef, len(list))
	for _, p := range list {
		out[p.ID] = providerRef{
			ID: p.ID.String(), Name: p.Name, Slug: p.Slug,
			Attribution: AttributionDTO{Text: p.AttributionText, URL: p.AttributionURL},
		}
	}
	return out, nil
}

// parseDateOrTime accepts an RFC3339 timestamp or a bare ISO date (UTC midnight).
func parseDateOrTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func boolPtr(b bool) *bool { return &b }
