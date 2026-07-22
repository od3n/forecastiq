# ForecastIQ — Approved Information Architecture

**Version**: 1.0 (UI ↔ Backend Reconciliation Board)
**Status**: Authoritative — supersedes conflicting IA elements in `docs/ui/03-operational-dashboard-design.md`; amends `docs/ui/02-ui-design-specification.md` §2–3 where noted
**Inputs**: `docs/ui/00-screen-inventory.md` (authoritative inventory); `docs/reviews/03-ui-backend-conflicts.md` (C-01, C-04, C-05, C-06, C-16); `docs/planning/03-ui-mvp-classification.md`

This is the approved IA for MVP implementation. It reconciles the Phase 0 Amendment screen inventory with the design exploration outputs. Anything not in this document is not approved for MVP.

---

## 1. Content Hierarchy (Approved)

```
ForecastIQ
├── Overview (S-01) — Rankings for selected location + horizon [+ observation context line]
├── Trends (S-04) — Metric trends over time
├── Forecast vs. Actual (S-05) — Overlay chart comparison [public via /forecast-comparison]
├── Location Detail (S-02) — All providers for one location
├── Provider Detail (S-03) — One provider across locations
├── Methodology (S-06) — Formulas, weights, worked example, versions
├── Settings (S-09) — Profile, API keys, preferences, danger zone [signed-in]
├── Admin (role-gated)
│   ├── Health (S-10) — Collector status grid + system health
│   ├── Providers (S-11) — Enable/disable, configurations
│   ├── Locations (S-12) — Add/edit/disable, dedup
│   ├── Schedules & Runs (S-13) — History, replay, recompute, trigger
│   └── Users & Audit (S-14) — Account management, event log
├── Onboarding (S-07) — First-use overlay [signed-in, first visit]
├── Auth (S-08) — Sign in / Sign up / Verify / Reset [Supabase-SDK-backed forms]
└── Error Pages (S-15) — 404, 500, 403, offline
```

**Removed from all MVP navigation** (per C-05, C-15): Alerts, Reports, Live Weather, Forecasts (raw), Integrations, Team (as separate item — folded into Admin › Users & Audit), notification bell, operational health strip on public screens.

**Reserved (not rendered)**: header layout slot for a future notification bell icon (doc 02 §9.2).

## 2. Navigation Model (Approved)

Single-row global header per `docs/ui/00-screen-inventory.md` §1 and `docs/ui/02-ui-design-specification.md` §3.1 (C-06):

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ [Logo: ForecastIQ]  Overview · Trends · Methodology · Admin* · Settings*   │
│                                                                              │
│ [Location Selector ▾]  [Horizon: +1h|+3h|+6h|+12h|+24h|+3d|+7d]  [Avatar] │
└─────────────────────────────────────────────────────────────────────────────┘
```

- `*` = role-gated (hidden when not authorized; server enforces regardless — see `docs/security/01-ui-authorization-matrix.md`)
- Location + horizon selectors: global controls persisted in URL (`?location_id=`, `?horizon_minutes=`)
- Avatar dropdown: Profile, Settings, Sign out (or "Sign in" when public)
- Admin sub-navigation: horizontal tab bar (Health | Providers | Locations | Schedules & Runs | Users & Audit) with breadcrumb per doc 02 §3.5
- Mobile: hamburger menu; selectors move to a collapsible filter bar (doc 02 §12.2)
- Sidebar navigation: **deferred** (C-06) — documented Level 3 evolution

## 3. URL Architecture (Approved, amends doc 02 §2.3)

| Screen | URL Pattern | Query Params |
|--------|-------------|--------------|
| Overview | `/` | `?location_id=&horizon_minutes=` |
| Trends | `/trends` | `?location_id=&horizon_minutes=&variable=&period=&aggregation=` |
| Forecast vs. Actual | `/forecast-vs-actual` | `?location_id=&date=&variable=&providers=` |
| Location Detail | `/locations/:id` | `?horizon_minutes=&period=` |
| Provider Detail | `/providers/:id` | `?location_id=&horizon_minutes=` |
| Methodology | `/methodology` | `#section-anchors` |
| Settings | `/settings` | `?tab=profile\|keys\|preferences\|danger` |
| Admin Health | `/admin/health` | `?provider_id=&status=` |
| Admin Providers | `/admin/providers` | — |
| Admin Locations | `/admin/locations` | — |
| Admin Schedules | `/admin/schedules` | `?provider_id=&status=` |
| Admin Users | `/admin/users` | `?tab=users\|audit&status=` |
| Auth | `/auth/signin`, `/auth/signup`, `/auth/verify`, `/auth/reset` | — |

No routes exist for: `/live-weather`, `/forecasts`, `/alerts`, `/reports`, `/integrations`, `/team`. Requests to these paths render S-15 (404).

## 4. Information Scent Principles (Approved, unchanged from doc 02 §2.2)

1. **Rankings are the front door.** Overview answers "Which provider is best for my location and horizon?" immediately.
2. **Evidence is one click away.** Every composite score links to its per-metric breakdown. Every metric links to methodology.
3. **Freshness is ambient.** Every data-bearing view shows last-updated without interaction.
4. **Admin is separate but accessible.** Role-gated section; never mixed into public data views. (Binding: no operational metrics on public screens — C-15.)
5. **Observations are evidence, not content.** (New, per C-01/C-04.) Observation values appear only as provenance-labeled context for comparison — never as a standalone weather display.

## 5. Cross-Linking Rules (Approved, amends doc 02 §3.3)

| From | To | Trigger |
|------|----|---------|
| Ranking row (S-01) | Provider Detail (S-03) | Click provider name |
| Composite score (S-01) | Breakdown panel (inline expand) | Click score or "Breakdown" |
| Any metric label | Methodology (S-06) with anchor | Click metric name or ⓘ icon |
| Methodology version badge | Methodology (S-06) | Click version string |
| Trends data point (S-04) | Forecast vs. Actual (S-05) | Click point → "View forecasts for this date" |
| Location context (S-01) | Location Detail (S-02) | Click location name |
| Admin Health row (S-10) | Admin Schedules (S-13) | Click collection ID |
| Admin Health stale cell | Runbook link (external) | Click "Runbook" per failure class |
| Observation context line (S-01/S-02) | Provenance tooltip | Hover/focus provenance badge |

## 6. Global Controls (Approved)

| Control | Data source | Rules |
|---------|-------------|-------|
| Location selector | `GET /locations?active=true` | Sorted by name; country flag code; URL-persisted; user default when signed in |
| Horizon selector | Static list [+1h, +3h, +6h, +12h, +24h, +3d, +7d] → minutes | URL param; default +24h (1440) |
| Date range (S-04) | Presets 7/30/90d + custom | URL params; default 30d; max 365d |
| Date (S-05) | Single-day picker | URL param; default today (location timezone) |
| Variable selector | temperature, precipitation (occurrence + amount), wind, humidity, pressure | Default temperature |
| Timezone display toggle | User preference (Settings) + `location.timezone` | BR-TZ rules; "Show times in my browser's timezone" (default off) |

**Removed**: global date control in header (C-16).

## 7. Attribution Footer (Approved, unchanged — BR-ATTR-01)

Present on every data-bearing page: provider attribution (configured per provider from `providers.attribution_text/url`, never hardcoded), observation provenance mix when relevant, methodology version, and the NP-01 disclaimer: "ForecastIQ measures forecast accuracy. We don't deliver weather forecasts."

## 8. Data Freshness Model (Approved, unchanged — BR-FRESH)

| State | Visual |
|-------|--------|
| `fresh` | Green dot; "Updated {relative} ago" |
| `delayed` | Amber badge "Data delayed" + last-updated |
| `stale` | Orange banner: "Data may be out of date — last updated {relative} ({absolute local})"; stale badge on rankings |
| `unavailable` | Red empty-state panel: "No data available — {reason if known}" |

Freshness is computed **server-side** and included in every time-sensitive payload (BR-FRESH-02). The UI never computes staleness from timestamps alone.

## 9. Permission Model (Approved, unchanged from doc 02 §2.5)

| Role | Access |
|------|--------|
| Public | S-01..S-06, S-15 — read-only |
| User (authenticated) | Public + S-07, S-09 + raw `/forecasts` `/observations` API queries |
| Admin | All + Admin section (S-10..S-14) |

Hiding navigation is UX only; every protected action is server-authorized (`docs/security/01-ui-authorization-matrix.md`).

## 10. Amendments to Prior Documents

| Document | Section | Amendment |
|----------|---------|-----------|
| `docs/ui/00-screen-inventory.md` | §1, §4 | Confirmed as authoritative; S-01 gains observation context line; S-03/S-05 API mappings corrected (C-08, C-19) — applied via `docs/ui/08-ui-backend-traceability.md` |
| `docs/ui/02-ui-design-specification.md` | §2.1 | Content hierarchy superseded by this document §1 (removals only; no additions) |
| `docs/ui/02-ui-design-specification.md` | §14.8 | Open items 1–5 resolved: chart library = implementation choice within budget; auth UI = custom forms + SDK (C-17); health refresh = 60s polling (DR-03); CSV format = ratified (DR-05); onboarding = localStorage (DR-04) |
| `docs/ui/03-operational-dashboard-design.md` | entire | Design exploration only; §1.3 sidebar, §9 screen inventory, and all Level-3 elements superseded by this document and `docs/planning/03-ui-mvp-classification.md`. Appendix A mock data = design fixture only (C-20). |
