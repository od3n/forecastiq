# ForecastIQ — Personas and User Journeys

**Version**: 1.0 (Phase 0 Amendment)
**Status**: Authoritative

---

## 1. MVP Personas

### P1 — Aisyah, the operator (primary, MVP)

The 1–2 engineer team itself. Runs the platform, curates locations, watches collector
health, validates data quality, answers "is the data trustworthy?"

- **Goals**: uninterrupted collection; quick detection of provider outages or schema
  drift; defensible numbers.
- **Frustrations**: silent data gaps; ambiguous metrics; infrastructure babysitting.
- **Surfaces**: admin section (health, schedules, providers, locations), ops dashboards
  (`/metrics`, logs), runbooks.

### P2 — Daniel, weather-curious individual (primary external)

Lives in Johor Bahru; plans photography and weekend hikes. Discovered that his weather
app "is always wrong about rain" and wants evidence about which source to trust.

- **Goals**: "which provider is best for rain tomorrow in JB?"; simple, honest answers
  with sample sizes.
- **Frustrations**: black-box scores; stale data presented as current; confusing stats.
- **Surfaces**: dashboard overview, location detail, forecast-vs-actual.

### P3 — Mei, data tinkerer (secondary)

Developer who wants the API for a personal project; values clean docs, stable
contracts, provenance fields.

- **Goals**: query rankings/metrics programmatically; understand methodology.
- **Surfaces**: OpenAPI docs, API keys, methodology page.

### P4 — Future business user (design-for only, Level 3)

Logistics planner. Needs multi-location views, exports, alerts, SLAs. Personas P4's
needs shape schema choices (workspace-ready) but are **not built** in MVP.

## 2. MVP User Journeys

### J1 — First visit (Daniel, unauthenticated)

1. Lands on dashboard → sees default location (Johor Bahru) overview with rankings.
2. Rankings show rank, composite score, sample size, coverage %, provenance badge,
   "last updated" freshness, methodology link.
3. If insufficient data: "Insufficient data to rank — collecting since {date}" (never a
   fake ranking).
4. Can browse without an account (read-only public data).

### J2 — Registration and personal use (Daniel)

1. "Sign in" → Supabase Auth: email + password, email verification on signup.
2. Post-login: saved default location preference, API key management in Settings.
3. Password reset via emailed link.

### J3 — Trust verification (Daniel/Mei)

1. Clicks methodology link on any ranking → methodology page (rendered from
   `docs/domain/03-metric-methodology.md` content) with weights, sample thresholds,
   version.
2. Clicks a ranking row → per-metric breakdown (temp MAE, rain F1, …) each with sample
   count and CI; "not significantly different" ties annotated.

### J4 — Operator health loop (Aisyah, daily)

1. Admin → Health: per-provider last-success time, circuit state, error counts,
   freshness states (fresh/delayed/stale/unavailable).
2. If stale: runbook link per failure class; one-click retry of a failed collection
   slot (idempotent — safe to retry).
3. Weekly: restore-test job result visible; payload volume usage.

### J5 — API consumption (Mei)

1. Settings → create API key (shown once, scoped).
2. `GET /api/v1/rankings?location_id=…&horizon_minutes=1440` → JSON with
   `methodology_version`, `sample_count`, `coverage`, `freshness`, request ID header.
3. Errors → RFC 7807 with request ID for support correlation.

### J6 — Location lifecycle (Aisyah)

1. Admin → Locations → add (name, lat/lon, timezone) → validation → active.
2. Next hourly cycle collects automatically; dashboard shows "collecting since" until
   first rankings mature (provisional at ≥10 pairs per variable, ranked at ≥30).
3. Disable (never delete) → collection stops; historical data remains.

## 3. Journey → Requirement Traceability

Journeys map to user stories in `docs/requirements/03-user-stories.md` (US IDs) and
screens in `docs/ui/00-screen-inventory.md`. Every screen state required by the UX
amendment (onboarding, empty, loading, partial failure, stale, etc.) is attached to the
journey step where it occurs — the UI data requirements document specifies the exact
data each state needs.
