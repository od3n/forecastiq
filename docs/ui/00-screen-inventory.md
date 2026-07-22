# ForecastIQ — Screen Inventory

**Version**: 1.1 (Phase 0 Amendment + UI ↔ Backend Reconciliation)
**Status**: Authoritative — input for the UI design phase; amended by the Reconciliation Board
**Resolves**: UX completeness amendment (all mandated states), ARB §15 UI gaps
**Reconciliation (v1.1)**: Confirmed as the authoritative screen inventory by the UI ↔ Backend
Reconciliation Board (`docs/reviews/04-ui-backend-reconciliation-report.md`). Amendments:
S-01 gains a provenance-labeled observation context line (C-01/C-04 — context, not a weather
display); S-03/S-05 API mappings corrected (§4); Alerts/Reports/Live Weather confirmed
absent from MVP navigation (C-05). `docs/ui/03-operational-dashboard-design.md` is
reclassified as design exploration; where it conflicts with this document, this document
governs. Binding companion documents: `docs/ui/04-approved-information-architecture.md`,
`05-screen-specifications.md`, `06-ui-state-contracts.md`, `08-ui-backend-traceability.md`.

This is a **specification, not a design**. No high-fidelity UI is produced in Phase 0.
A design agent can work from this document without inventing business rules: every
state, its trigger, and its required content are defined here and in
`docs/ui/01-ui-data-requirements.md`.

---

## 1. Global Navigation

Header nav (single row): **Overview · Trends · Methodology · Admin (role-gated) ·
Settings · Sign in/Account**. Location and horizon selectors are global controls
persisted in the URL. Attribution footer on every data-bearing page (BR-ATTR-01).

## 2. Screens

| # | Screen | Purpose | Auth |
|---|--------|---------|------|
| S-01 | Overview | Rankings for selected location + horizon; headline freshness; quick stats | public |
| S-02 | Location Detail | All providers compared for one location; per-variable tables; data window status | public |
| S-03 | Provider Detail | One provider's performance across locations/horizons; collection health summary | public |
| S-04 | Trends | Metric trends over time, multi-provider overlay, aggregation selector | public |
| S-05 | Forecast vs. Actual | Overlay chart for a date + variable + provider set; error bands | public |
| S-06 | Methodology | Human-readable methodology (formulas, weights, thresholds, versions, worked example) | public |
| S-07 | Onboarding (first-use) | What we measure / don't measure; pick default location; link to methodology | signed-in, first visit |
| S-08 | Auth pages | Sign in / Sign up / Verify email / Reset password / Update password (managed-auth hosted or embedded flows) | public |
| S-09 | Settings | Profile, default location, timezone display toggle, API keys CRUD, account danger zone (export/delete) | signed-in |
| S-10 | Admin › Health | Per provider-location: last success, circuit state, error counts, freshness grid; retry actions | admin |
| S-11 | Admin › Providers | Enable/disable providers; edit configurations (schedule, credential status) | admin |
| S-12 | Admin › Locations | Add/edit/disable locations; dedup warnings; per-location collection window | admin |
| S-13 | Admin › Schedules & Runs | Schedule editor; recent run history with statuses; replay + recompute actions | admin |
| S-14 | Admin › Users & Audit | User list (disable/delete), audit event log | admin |
| S-15 | Error pages | 404, 500, 403, offline/network-loss | all |

## 3. Mandatory States per Screen (UX amendment)

Every data-bearing screen (S-01..S-05, S-10..S-13) implements **all** of these:

| State | Trigger | Required content |
|-------|---------|------------------|
| **Loading** | Data fetch in flight | Skeleton layout matching final layout (no layout shift); previous data dimmed on refetch |
| **Empty — no locations exist** | `GET /locations` returns none (S-01 default view) | "No locations monitored yet" + (admin) add-location CTA; never a broken chart |
| **Empty — location has no data yet** | Location exists, zero collections | "Collecting since {date} — first data appears within ~1 h" with progress hint |
| **Insufficient data for ranking** | Cell `unranked`/`provisionally_ranked` | Explicit label: "Insufficient data ({n}/{threshold} samples)" or "Provisional — {n} samples"; provisional badge distinct from ranked |
| **Partial provider failure** | `warnings[]` in API response | Per-provider badge "temporarily unavailable" on affected rows; unaffected providers render normally |
| **Observation unavailable** | No observation source data for location/period | "Ground truth unavailable for this period — metrics not computed" + provenance note |
| **Stale data** | `freshness.state = stale/delayed` | Banner: "Data may be out of date — last updated {relative + absolute local time}"; stale badge on rankings |
| **Full API failure** | Network error / 5xx on all fetches | Error panel with retry button; cached data (if any) shown with explicit "cached {time}" label |
| **Permission denied** | 403 on admin screens | "You need administrator access" + sign-in switch hint |
| **Empty alerts state** | N/A in MVP (alerts deferred) — reserved pattern documented for Level 3 | — |

Additional mandated behaviors:

| Behavior | Rule |
|----------|------|
| **First-use onboarding** (S-07) | Shown once per account; dismissible; re-openable from Settings |
| **Export flow** | "Export CSV" button on S-02/S-04/S-05; exports current filters; metadata header rows (methodology, period, provenance); button disabled with tooltip when no data |
| **Data freshness display** | Every data view shows `last updated` (relative + absolute, location timezone per BR-TZ) and freshness state color |
| **Methodology explanation** | Composite scores and every metric label link to S-06 anchors; S-06 shows current methodology_version + weights_version |
| **Sample-size display** | Every metric and ranking shows n; below threshold, n is highlighted |
| **Timezone display** | Per BR-TZ-02..05: location timezone default, browser toggle, explicit zone labels |
| **Chart interaction** | MVP: hover tooltips with exact values + CI; zoom/pan deferred (documented); keyboard-accessible legends |
| **Accessibility** | WCAG 2.1 AA: contrast ≥ 4.5:1, focus order, ARIA labels on charts (data-table fallback), reduced-motion respect |

## 4. Screen → API Mapping

| Screen | Primary endpoints |
|--------|-------------------|
| S-01 | `/rankings` (incl. `observation_context` block), `/locations` |
| S-02 | `/accuracy/summary`, `/rankings`, `/observations` (provenance) |
| S-03 | `/accuracy`, `/accuracy/summary?provider_id=` (collection window + grid; C-08), `/rankings` |
| S-04 | `/accuracy` (aggregated) |
| S-05 | `/forecast-comparison` (public, bounded — C-19; raw `/forecasts`+`/observations` remain user+) |
| S-06 | `/rankings/methodology` + static content |
| S-09 | `/me`, `/api-keys`, `/me/export`, `/me` (DELETE) |
| S-10..S-14 | `/admin/*` |

## 5. Deferred UI (Level 3, documented not designed)

Heatmap, alert preferences, notification center, organization/workspace switcher,
billing pages, mobile-native layouts, chart zoom/pan/brush, dark mode.
