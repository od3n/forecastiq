# ForecastIQ — UI Data Requirements

**Version**: 1.0 (Phase 0 Amendment)
**Status**: Authoritative — what data each screen/state must receive

This document ensures the design and frontend implementation phases never have to
invent business rules: every displayed value has a defined source, format, and
fallback.

---

## 1. Global Controls

| Control | Data source | Rules |
|---------|-------------|-------|
| Location selector | `GET /locations?active=true` | Sorted by name; shows country flag code; selection persisted in URL (`?location_id=`) and as user default when signed in |
| Horizon selector | Static list [+1h, +3h, +6h, +12h, +24h, +3d, +7d] → minutes | URL param `horizon_minutes`; default +24h |
| Date range (Trends/FvA) | Presets 7/30/90d + custom | URL params; default 30d; max 365d |
| Variable selector | temperature, precipitation (occurrence + amount), wind, humidity, pressure | Default temperature |
| Timezone display | User preference (Settings) + location.timezone | BR-TZ rules; toggle labeled "Show times in my browser's timezone" |

## 2. Per-Screen Data Requirements

### S-01 Overview
- Ranking rows: rank, provider name + attribution link, composite score (0–1, 3 dp),
  status badge (ranked/provisional/unranked + reason), sample count, coverage %,
  CI (shown on hover + "not significantly different" grouping), freshness state.
- Header: location name + timezone label, "last updated {relative} ({absolute local})",
  data-window ("based on {period} of data").
- Quick stats: cells ranked/provisional/unranked counts; observation provenance badge.

### S-02 Location Detail
- Per-variable metric table per provider: value ± CI, sample count, null shown as "—"
  with tooltip "no events in period" (never 0).
- Occurrence metrics include the imbalance warning text next to occurrence_agreement.
- Collection window per provider: first/last snapshot timestamps, coverage %.

### S-03 Provider Detail
- Cross-location metric grid; per-horizon composite; collection reliability vs.
  coverage distinction explained inline (our failures vs. provider gaps).

### S-04 Trends
- Series per provider: metric value line + CI band; x-axis bucketed per BR-TZ-05;
  aggregation selector (daily/weekly/monthly); sample count per point on hover;
  points with n < threshold drawn hollow with "provisional" legend entry.

### S-05 Forecast vs. Actual
- Forecast lines per provider (issued_at fixed, target axis), observation line with
  provenance badge; error band (±MAE of the selected period); missing observation
  gaps rendered as breaks, not interpolated.

### S-06 Methodology
- Rendered from `/rankings/methodology` + static prose: formulas (plain language +
  math), default weights table with rationale, thresholds (30/10), coverage penalty,
  statuses, tie handling, worked example, version numbers, change history.

### S-09 Settings
- API keys: prefix, name, scopes, created, last used, status; create dialog shows key
  once with copy button + warning; revoke confirmation.
- Account: export (async, email/download link), delete (type-to-confirm, states what is
  deleted vs. retained per AUTH-09).

### S-10 Admin Health
- Grid rows per provider-location: last success (relative + absolute), freshness state
  color, circuit state, consecutive failures, last error code/message, retry button
  (disabled while circuit open; label explains), payload volume usage %.

### S-12 Admin Locations
- Form: name, lat, lon, timezone (IANA picker), with inline validation; dedup warning
  (BR-LOC-01) shown before submit with link to the near-duplicate.

### S-13 Admin Schedules & Runs
- Run history: collection id, provider, location, status (with counts:
  stored/dedup/invalid), latency, error code; replay action per stored payload;
  recompute action with scope picker + confirmation.

## 3. Number Formatting Rules (UI)

| Kind | Format |
|------|--------|
| Composite score | 0.000 (3 dp) |
| Error metrics (°C, mm, m/s) | 2 dp + unit |
| Ratios (precision/recall/F1/FAR) | percentage, 1 dp (e.g., 88.9%) |
| Coverage/reliability | percentage, 0 dp |
| Sample counts | integer; highlighted when below active threshold |
| Timestamps | "Jul 22, 18:00 MYT" (location tz) or browser tz per toggle; relative "2 h ago" alongside |
| CI | "±0.09" or band; explained in tooltip |

## 4. Copy Rules (tone and honesty)

- Never say "accuracy" alone; say "accuracy metrics" with the specific metric named.
- Insufficient data copy always includes the count vs. threshold and what happens next
  ("keeps collecting").
- Stale copy always includes last-updated time.
- Attribution copy matches provider ToS text requirements exactly (configured per
  provider, not hardcoded).
