# ForecastIQ — Observation Collection Workflow (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: OC-01..OC-06; BR-OBS-01..04; ADR-003; domain model §7

---

## 1. Sequence Diagram

```mermaid
sequenceDiagram
    participant SCH as Scheduler
    participant DB as PostgreSQL
    participant OC as ObservationCollector
    participant SRC as Open-Meteo Historical

    SCH->>DB: claim observation slot (slot_time = HH:05, per location)
    DB-->>SCH: slot (location)
    SCH->>OC: collect(location)
    OC->>SRC: GET historical (last 2h window, hourly)
    alt success
        SRC-->>OC: JSON (per-hour values + provenance hints)
        OC->>OC: per row: range validation (OC-04)
        OC->>OC: provenance tagging (observation_type)
        OC->>OC: condition mapping (canonical taxonomy)
        OC->>DB: BEGIN tx
        OC->>DB: INSERT observations ON CONFLICT (source, location_id, observed_at) DO NOTHING
        OC->>DB: detect corrections (same source/time, different values vs stored)
        opt correction detected
            OC->>DB: INSERT new row (quality_flag=corrected)
            OC->>DB: UPDATE old row superseded_observation_id
            OC->>DB: emit observation.corrected
        end
        OC->>DB: emit observation.collected
        OC->>DB: COMMIT
        OC->>DB: mark slot completed
    else failure
        SRC-->>OC: error/timeout
        OC->>OC: retry backoff (1,2,4,8,16s; max 5)
        OC->>DB: mark slot failed + run row
    end
```

## 2. Collection Model Decisions

| Question | Decision | Rationale |
|----------|----------|-----------|
| Polled / batched / on-demand / backfilled? | **Polled hourly** with 2 h backfill window per call | Source publishes with delay; window dedups naturally via uniqueness; no push API exists |
| Parent collection entity? | **None** (reconciliation verdict) | Observations are small (240 rows/day); health derives from observation aggregates; no replay need (source is re-queryable) |
| Raw payload storage? | **Not for observations** | Small, re-fetchable, no adapter-replay need; checksums unnecessary (source is truth reference) |
| Source priority | Single source (openmeteo_historical) in MVP; provenance rank ready for multi-source (BR-MATCH-03) | ADR-003 |

## 3. Step-by-Step Specification

| # | Step | Specification |
|---|------|---------------|
| 1 | Trigger | Scheduler slot at **:05 past each hour** per active location (OC-01) — 5 min allows source publication delay |
| 2 | Window | Request last 2 hours (handles late publication + source-side corrections; overlap rows deduped by uniqueness) |
| 3 | Source call | Open-Meteo Historical API: lat/lon, hourly variables, `past_days=0` + explicit start/end (UTC) |
| 4 | Provenance tagging | `source = 'openmeteo_historical'`; `observation_type`: per API-exposed provenance where available; **default `reanalysis`** where not exposed per-variable (documented binding assumption); provider current-weather (if ever added) = `provider_estimated` only, never primary |
| 5 | Range validation | OC-04 ranges: temp −90..60 °C, humidity 0..100 %, wind 0..120 m/s, pressure 870..1084 hPa, precip 0..500 mm/h; violations → `quality_flag = suspect` (stored, excluded from metrics, counted in metric `observations_suspect_total`) |
| 6 | Normalization | UTC (source returns UTC); units canonical (°C, mm, m/s, hPa); condition codes mapped to taxonomy |
| 7 | Deduplication | `ON CONFLICT (source, location_id, observed_at) DO NOTHING` (OC-03) |
| 8 | Correction handling | See §4 |
| 9 | Event emission | `observation.collected` (always); `observation.corrected` (when correction detected) |
| 10 | Freshness | `observation_freshness_age_seconds` gauge per location; BR-FRESH thresholds (fresh < 90 min) |

## 4. Correction Handling

Open-Meteo Historical may republish a value for an already-collected hour (model assimilation updates):

1. Collector fetches window; for each (source, location, observed_at) that already exists, compare weather values.
2. **Values differ beyond ε** (temp > 0.1 °C, precip > 0.05 mm, etc.) → correction:
   - INSERT new observation row: `quality_flag = 'corrected'`, new values.
   - UPDATE old row: `superseded_observation_id = new.id` (the only permitted observation mutation).
   - Emit `observation.corrected` → analysis rematches affected pairs + recomputes affected metrics/rankings as new rows (BR-INV-01, ≤ 1 h).
3. Values equal → dedup (DO NOTHING), no correction.

**Correction cascade:** rematch creates new MatchedEvaluation rows (old retained); affected AccuracyMetric + ProviderRanking rows recomputed with `superseded_by` links; audit event logged; completion within 2 batch cycles (BR-INV-02).

## 5. Late-Arriving Observations

- The 2 h window captures source publication delays up to 2 h.
- Observations arriving later (source outage recovery): next successful call backfills; matching engine pairs them with existing (still unmatched, within 30 d window) snapshots in the next batch.
- Metrics for affected periods recomputed as new rows (late arrival = same path as correction, BR-INV-01).
- Rankings never corrupted by lateness: they reflect the latest batch; freshness labels communicate lag.

## 6. Missing-Data States

| Situation | Representation |
|-----------|----------------|
| Hour with no observation (source gap) | No row; matching leaves snapshots unmatched; coverage unaffected (provider-side); observation freshness degrades per BR-FRESH |
| Variable missing in an otherwise valid observation | Row stored with NULL variable; per-variable eligibility excludes it from that variable's metrics only (methodology §3 rule 3) |
| Source outage > 24 h | Freshness `unavailable`; UI explicit empty state; rankings serve last batch with staleness warning |
| Suspect values | Row stored (quality_flag=suspect), excluded from all metrics, counted operationally |

## 7. Quality Spike Gate (D-06, pre-launch)

Before ranking publication at launch: compare Open-Meteo Historical reanalysis against available METMalaysia station bulletins/literature for JB (temperature bias, rain occurrence agreement). Gate outcome:
- Acceptable → publish rankings with provenance disclosure (weight 0.8 reanalysis).
- Unacceptable for rain occurrence → adjust provenance mix/weighting + heavier caveats (ADR-003 migration trigger).
This is a **product gate**, not an architecture change — the pipeline is identical either way.

## 8. Observability

- `observations_collected_total{source, location_id}`, `observations_suspect_total{source, reason}`, `observation_freshness_age_seconds{location_id}`.
- Alert: observation freshness > 240 min (stale threshold) for any active location → warning.
- S-10 health: per-location last success, suspect count 24 h, locations covered.

## 9. Cross-Reference

- Matching consumption: `docs/workflows/03-matching.md`
- Correction recomputation: `docs/workflows/04-evaluation-and-ranking.md` §5
- ADR-003 (source strategy); observation quality risk R-03
