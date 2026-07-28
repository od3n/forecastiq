// Bulk row generation via COPY (WP-26b functional seeder). Every writer
// streams rows lazily through pgx.CopyFromFunc — nothing is buffered, so the
// extended preset (~35M snapshots) runs in bounded memory.
//
// Volume model (docs/testing/04-performance-testing.md §3): hourly collection
// runs × 104 forecast rows per run (hourly to +72h, then 3-hourly to +168h),
// observations with a full correction pass (original superseded + correcting
// row), and matched pairs to BOTH rows (original pair retained on rematch,
// workflow §4) ⇒ matches ≈ 2× snapshots.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	analysisdomain "github.com/forecastiq/forecastiq/internal/analysis/domain"
	collectiondomain "github.com/forecastiq/forecastiq/internal/collection/domain"
)

// targetOffsets returns the 104 forecast target offsets (hours): hourly to
// +72h, then 3-hourly to +168h. 72 + 32 = 104 = forecastRowsPerRun.
func targetOffsets() []int {
	out := make([]int, 0, forecastRowsPerRun)
	for h := 1; h <= 72; h++ {
		out = append(out, h)
	}
	for h := 75; h <= 168; h += 3 {
		out = append(out, h)
	}
	return out
}

// canonicalHorizons are the UI horizon options (doc 02 §3.1) used for the
// pre-aggregated metric/ranking backfill.
var canonicalHorizons = []int{60, 180, 360, 720, 1440, 4320, 10080}

// dataset is the fully-resolved generation plan.
type dataset struct {
	seed    int64
	provs   []providerRef
	nLoc    int
	days    int
	start   time.Time // first issuance hour (UTC, hour-truncated)
	end     time.Time // exclusive issuance bound == last observed hour
	offsets []int
}

func newDataset(seed int64, provs []providerRef, nLoc, days int, now time.Time) dataset {
	end := now.UTC().Truncate(time.Hour)
	return dataset{
		seed: seed, provs: provs, nLoc: nLoc, days: days,
		start: end.Add(-time.Duration(days) * 24 * time.Hour), end: end,
		offsets: targetOffsets(),
	}
}

func (d dataset) issuanceHours() int { return d.days * 24 }

// condition maps precipitation to a canonical condition code.
func condition(precipMM float64) string {
	switch {
	case precipMM >= 5:
		return collectiondomain.ConditionHeavyRain
	case precipMM > 0.1:
		return collectiondomain.ConditionRain
	default:
		return collectiondomain.ConditionPartlyCloudy
	}
}

// copyCollections writes one success collection per (provider, location,
// issuance hour).
func copyCollections(ctx context.Context, conn *pgx.Conn, d dataset) (int64, error) {
	hours := d.issuanceHours()
	total := len(d.provs) * d.nLoc * hours
	i := 0
	return conn.CopyFrom(ctx, pgx.Identifier{"forecast_collections"},
		[]string{"id", "provider_id", "location_id", "provider_configuration_id",
			"requested_at", "completed_at", "collection_status", "response_status_code",
			"response_latency_ms", "records_received", "snapshots_stored",
			"snapshots_deduplicated", "snapshots_invalid", "schema_version", "adapter_version"},
		pgx.CopyFromFunc(func() ([]any, error) {
			if i >= total {
				return nil, nil
			}
			p := i / (d.nLoc * hours)
			rem := i % (d.nLoc * hours)
			l := rem / hours
			h := rem % hours
			i++
			requested := d.start.Add(time.Duration(h) * time.Hour)
			issuedUnix := requested.Unix() / 3600
			latency := 80 + int(hash(d.seed, 0x4c415459, uint64(p), uint64(l), uint64(issuedUnix))%700)
			return []any{
				collectionID(p, l, issuedUnix), d.provs[p].ProviderID, perfLocationID(l), d.provs[p].ConfigID,
				requested, requested.Add(time.Duration(latency) * time.Millisecond), "success", 200,
				latency, forecastRowsPerRun, forecastRowsPerRun, 0, 0, "perf-synthetic-v1", "1.0.0-perf",
			}, nil
		}))
}

// copySnapshots writes the 104 forecast rows of every collection run.
func copySnapshots(ctx context.Context, conn *pgx.Conn, d dataset) (int64, error) {
	hours := d.issuanceHours()
	perRun := len(d.offsets)
	total := len(d.provs) * d.nLoc * hours * perRun
	i := 0
	return conn.CopyFrom(ctx, pgx.Identifier{"forecast_snapshots"},
		[]string{"id", "forecast_collection_id", "provider_id", "location_id",
			"issued_at", "target_time", "forecast_horizon_minutes",
			"temperature_c", "precipitation_probability", "precipitation_amount_mm",
			"humidity_pct", "wind_speed_ms", "pressure_hpa",
			"canonical_condition_code", "condition_taxonomy_version"},
		pgx.CopyFromFunc(func() ([]any, error) {
			if i >= total {
				return nil, nil
			}
			p := i / (d.nLoc * hours * perRun)
			rem := i % (d.nLoc * hours * perRun)
			l := rem / (hours * perRun)
			rem = rem % (hours * perRun)
			h := rem / perRun
			o := rem % perRun
			i++
			issued := d.start.Add(time.Duration(h) * time.Hour)
			target := issued.Add(time.Duration(d.offsets[o]) * time.Hour)
			iu, tu := issued.Unix()/3600, target.Unix()/3600
			f := forecastFor(d.seed, p, l, iu, tu)
			return []any{
				snapshotID(p, l, iu, tu), collectionID(p, l, iu), d.provs[p].ProviderID, perfLocationID(l),
				issued, target, d.offsets[o] * 60,
				f.TempC, f.PrecipProb, f.PrecipMM,
				f.HumidityPct, f.WindMS, f.PressureHPA,
				condition(f.PrecipMM), collectiondomain.ConditionTaxonomyVersion,
			}, nil
		}))
}

// copyObservations writes, per location-hour, the original (superseded) row
// and the correcting live row — the reanalysis finalization pass (workflow §4).
func copyObservations(ctx context.Context, conn *pgx.Conn, d dataset) (int64, error) {
	hoursInclusive := d.issuanceHours() + 1 // [start, end]
	total := d.nLoc * hoursInclusive * 2
	i := 0
	return conn.CopyFrom(ctx, pgx.Identifier{"observations"},
		[]string{"id", "location_id", "source", "observation_type", "observed_at",
			"temperature_c", "humidity_pct", "wind_speed_ms", "pressure_hpa", "precipitation_mm",
			"canonical_condition_code", "quality_flag", "superseded_observation_id"},
		pgx.CopyFromFunc(func() ([]any, error) {
			if i >= total {
				return nil, nil
			}
			l := i / (hoursInclusive * 2)
			rem := i % (hoursInclusive * 2)
			h := rem / 2
			corrected := rem%2 == 1
			i++
			at := d.start.Add(time.Duration(h) * time.Hour)
			hu := at.Unix() / 3600
			w := trueWeather(d.seed, l, hu)

			id := observationID(l, hu, corrected)
			flag := "valid"
			var supersededBy any
			temp := w.TempC
			if corrected {
				temp = round2(w.TempC + correctionDelta(d.seed, l, hu))
				flag = "corrected"
				if suspectObservation(d.seed, l, hu) {
					flag = "suspect"
				}
			} else {
				supersededBy = observationID(l, hu, true)
			}
			return []any{
				id, perfLocationID(l), "openmeteo_historical", "reanalysis", at,
				temp, w.HumidityPct, w.WindMS, w.PressureHPA, w.PrecipMM,
				condition(w.PrecipMM), flag, supersededBy,
			}, nil
		}))
}

// copyPairs writes the matched_evaluations rows: every snapshot whose target
// hour has been observed pairs with the original observation (first match) and
// with the correcting row (rematch; original pair retained). Rematch pairs to
// suspect live rows are skipped (matching excludes suspect observations).
func copyPairs(ctx context.Context, conn *pgx.Conn, d dataset) (int64, error) {
	hours := d.issuanceHours()
	perRun := len(d.offsets)
	totalSnaps := len(d.provs) * d.nLoc * hours * perRun
	now := time.Now().UTC()
	i := 0
	corrected := false // second pair of the current snapshot pending?
	var pending []any
	return conn.CopyFrom(ctx, pgx.Identifier{"matched_evaluations"},
		[]string{"id", "forecast_snapshot_id", "observation_id", "provider_id", "location_id",
			"forecast_horizon_minutes", "target_time", "match_rule", "time_delta_minutes", "computed_at"},
		pgx.CopyFromFunc(func() ([]any, error) {
			if corrected {
				corrected = false
				return pending, nil
			}
			for i < totalSnaps {
				p := i / (d.nLoc * hours * perRun)
				rem := i % (d.nLoc * hours * perRun)
				l := rem / (hours * perRun)
				rem = rem % (hours * perRun)
				h := rem / perRun
				o := rem % perRun
				i++
				issued := d.start.Add(time.Duration(h) * time.Hour)
				target := issued.Add(time.Duration(d.offsets[o]) * time.Hour)
				if target.After(d.end) {
					continue // not yet observed
				}
				iu, tu := issued.Unix()/3600, target.Unix()/3600
				snapID := snapshotID(p, l, iu, tu)
				horizon := d.offsets[o] * 60
				row := func(corr bool) []any {
					c := uint64(0)
					if corr {
						c = 1
					}
					return []any{
						keyedID("pair", uint64(p), uint64(l), uint64(iu), uint64(tu), c),
						snapID, observationID(l, tu, corr), d.provs[p].ProviderID, perfLocationID(l),
						horizon, target, "exact_hour", 0, now,
					}
				}
				if !suspectObservation(d.seed, l, tu) {
					pending = row(true)
					corrected = true
				}
				return row(false), nil
			}
			return nil, nil
		}))
}

// metricPlan enumerates the pre-aggregated daily accuracy_metrics backfill:
// per cell (provider × location × canonical horizon) × day, the same 27 rows
// the aggregation engine writes (4 continuous × 3 + rain 2 + categorical 6 +
// brier 1 + coverage 5 + reliability 1).
type metricRow struct {
	variable   string
	metricType string
	bounded    bool // value ∈ [0,1] (ratios/probabilistic)
	hasCI      bool
}

var metricPlan = func() []metricRow {
	var rows []metricRow
	for _, v := range []string{analysisdomain.VarTemperature, analysisdomain.VarWindSpeed,
		analysisdomain.VarHumidity, analysisdomain.VarPressure} {
		rows = append(rows,
			metricRow{v, analysisdomain.MetricMAE, false, true},
			metricRow{v, analysisdomain.MetricRMSE, false, true},
			metricRow{v, analysisdomain.MetricBias, false, true})
	}
	rows = append(rows,
		metricRow{analysisdomain.VarPrecipitation, analysisdomain.MetricRainMAEAll, false, true},
		metricRow{analysisdomain.VarPrecipitation, analysisdomain.MetricRainMAEWet, false, true},
		metricRow{analysisdomain.VarPrecipitation, analysisdomain.MetricRecall, true, true},
		metricRow{analysisdomain.VarPrecipitation, analysisdomain.MetricPrecision, true, true},
		metricRow{analysisdomain.VarPrecipitation, analysisdomain.MetricF1, true, true},
		metricRow{analysisdomain.VarPrecipitation, analysisdomain.MetricFAR, true, true},
		metricRow{analysisdomain.VarPrecipitation, analysisdomain.MetricThreatScore, true, true},
		metricRow{analysisdomain.VarPrecipitation, analysisdomain.MetricOccurrenceAgreement, true, true},
		metricRow{analysisdomain.VarPrecipitation, analysisdomain.MetricBrier, true, true})
	for _, v := range []string{analysisdomain.VarTemperature, analysisdomain.VarWindSpeed,
		analysisdomain.VarHumidity, analysisdomain.VarPressure, analysisdomain.VarPrecipitation} {
		rows = append(rows, metricRow{v, analysisdomain.MetricCoverage, true, false})
	}
	return append(rows, metricRow{analysisdomain.VarAll, analysisdomain.MetricReliability, true, false})
}()

// metricValue produces a deterministic plausible value + CI for one metric row.
func metricValue(seed int64, p, l, horizon, day int, m metricRow) (value, ciLo, ciHi float64) {
	h := hash(seed, 0x4d455452, uint64(p), uint64(l), uint64(horizon), uint64(day),
		hash(seed, uint64(len(m.variable)), uint64(len(m.metricType))))
	n := unit(h)
	horizonH := float64(horizon) / 60
	switch {
	case m.bounded:
		// Skill ratios decay slightly with horizon; provider 0 is best.
		value = clamp(0.92-0.015*horizonH/24-0.05*float64(p)-0.1*n, 0.05, 0.999)
		half := 0.02 + 0.03*unit(splitmix64(h+1))
		ciLo, ciHi = clamp(value-half, 0, 1), clamp(value+half, 0, 1)
	case m.metricType == analysisdomain.MetricBias:
		value = round4((n - 0.5) * (0.4 + 0.02*horizonH))
		half := 0.05 + 0.1*unit(splitmix64(h+1))
		ciLo, ciHi = value-half, value+half
	default: // mae / rmse / rain_mae*
		value = round4((0.4 + 0.02*horizonH) * (1 + 0.25*float64(p)) * (0.8 + 0.4*n))
		if m.metricType == analysisdomain.MetricRMSE {
			value = round4(value * 1.27)
		}
		half := value * (0.08 + 0.08*unit(splitmix64(h+1)))
		ciLo, ciHi = value-half, value+half
	}
	return round4(value), round4(ciLo), round4(ciHi)
}

// copyMetrics backfills daily accuracy_metrics rows for every cell-day.
func copyMetrics(ctx context.Context, conn *pgx.Conn, d dataset) (int64, error) {
	perCellDay := len(metricPlan)
	cells := len(d.provs) * d.nLoc * len(canonicalHorizons)
	total := cells * d.days * perCellDay
	dayZero := time.Date(d.start.Year(), d.start.Month(), d.start.Day(), 0, 0, 0, 0, time.UTC)
	now := time.Now().UTC()
	i := 0
	return conn.CopyFrom(ctx, pgx.Identifier{"accuracy_metrics"},
		[]string{"id", "provider_id", "location_id", "horizon_minutes", "variable", "metric_type",
			"value", "ci_lower", "ci_upper", "sample_count", "methodology_version",
			"period_start", "period_end", "calculated_at"},
		pgx.CopyFromFunc(func() ([]any, error) {
			if i >= total {
				return nil, nil
			}
			idx := i
			i++
			planIdx := idx % perCellDay
			m := metricPlan[planIdx]
			idx /= perCellDay
			day := idx % d.days
			idx /= d.days
			hz := canonicalHorizons[idx%len(canonicalHorizons)]
			idx /= len(canonicalHorizons)
			l := idx % d.nLoc
			p := idx / d.nLoc

			value, ciLo, ciHi := metricValue(d.seed, p, l, hz, day, m)
			sample := 24
			if hz > 72*60 {
				sample = 8
			}
			var lo, hi any
			if m.hasCI {
				lo, hi = ciLo, ciHi
			}
			ps := dayZero.AddDate(0, 0, day)
			return []any{
				keyedID("met", uint64(p), uint64(l), uint64(hz), uint64(day), uint64(planIdx)),
				d.provs[p].ProviderID, perfLocationID(l), hz, m.variable, m.metricType,
				value, lo, hi, sample, analysisdomain.MethodologyVersion,
				ps, ps.AddDate(0, 0, 1), now,
			}, nil
		}))
}

// copyRankings publishes live provider_rankings rows for the last full day per
// (location × canonical horizon × provider), with a component breakdown in the
// engine's ComponentScore shape.
func copyRankings(ctx context.Context, conn *pgx.Conn, d dataset) (int64, error) {
	total := d.nLoc * len(canonicalHorizons) * len(d.provs)
	now := time.Now().UTC()
	dayEnd := time.Date(d.end.Year(), d.end.Month(), d.end.Day(), 0, 0, 0, 0, time.UTC)
	dayStart := dayEnd.AddDate(0, 0, -1)
	i := 0
	return conn.CopyFrom(ctx, pgx.Identifier{"provider_rankings"},
		[]string{"id", "provider_id", "location_id", "horizon_minutes",
			"composite_score", "ci_lower", "ci_upper", "ranking_status", "sample_count",
			"coverage", "reliability", "component_scores", "methodology_version",
			"weights_version", "horizon_profile", "period_start", "period_end", "calculated_at"},
		pgx.CopyFromFunc(func() ([]any, error) {
			if i >= total {
				return nil, nil
			}
			idx := i
			i++
			p := idx % len(d.provs)
			idx /= len(d.provs)
			hz := canonicalHorizons[idx%len(canonicalHorizons)]
			l := idx / len(canonicalHorizons)

			composite := 0.0
			scores := make([]analysisdomain.ComponentScore, 0, len(analysisdomain.Components))
			for ci, c := range analysisdomain.Components {
				h := hash(d.seed, 0x52414e4b, uint64(p), uint64(l), uint64(hz), uint64(ci))
				norm := clamp(0.98-0.06*float64(p)-0.15*unit(h), 0.3, 1)
				raw := round4((0.4 + 0.02*float64(hz)/60) * (1 + 0.25*float64(p)) * (0.8 + 0.4*unit(splitmix64(h+1))))
				v, nv := raw, round4(norm)
				scores = append(scores, analysisdomain.ComponentScore{
					Component: c.Key, Value: &v, Normalized: &nv, Weight: c.Weight,
				})
				composite += c.Weight * norm
			}
			composite = round4(clamp(composite, 0, 1))
			cs, err := json.Marshal(scores)
			if err != nil {
				return nil, fmt.Errorf("component scores: %w", err)
			}
			coverage := round4(0.93 + 0.06*unit(hash(d.seed, 0x434f56, uint64(p), uint64(l), uint64(hz))))
			reliability := round4(0.95 + 0.045*unit(hash(d.seed, 0x52454c, uint64(p), uint64(l), uint64(hz))))
			return []any{
				keyedID("rank", uint64(p), uint64(l), uint64(hz)),
				d.provs[p].ProviderID, perfLocationID(l), hz,
				composite, round4(clamp(composite-0.015, 0, 1)), round4(clamp(composite+0.015, 0, 1)),
				analysisdomain.StatusRanked, 400 + int(hash(d.seed, 0x53414d, uint64(p), uint64(l), uint64(hz))%80),
				coverage, reliability, cs, analysisdomain.MethodologyVersion,
				analysisdomain.WeightsVersion, analysisdomain.ProfileUniform, dayStart, dayEnd, now,
			}, nil
		}))
}
