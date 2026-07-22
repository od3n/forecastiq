# ForecastIQ — UI ↔ Backend Conflict Register

**Version**: 1.0 (UI ↔ Backend Reconciliation Board)
**Status**: Authoritative — all conflicts resolved or owned
**Inputs**: Phase 0 Amendment baseline; `docs/ui/00-screen-inventory.md`; `docs/ui/01-ui-data-requirements.md`; `docs/ui/02-ui-design-specification.md`; `docs/ui/03-operational-dashboard-design.md`
**Process**: Each conflict is assigned an ID, described from both sides, evaluated across UX / engineering / data / security / operations, resolved with a decision, and assigned an owner. No blocking conflict remains unowned.

---

## Preamble: Document Authority

Two UI design artifacts exist with **different screen inventories, navigation models, and product framing**:

| Document | Authority | Role in reconciliation |
|----------|-----------|------------------------|
| `docs/ui/00-screen-inventory.md` + `01-ui-data-requirements.md` (Phase 0 Amendment) | **Authoritative** (amended Phase 0 baseline) | Product source of truth for screens, states, and API mapping |
| `docs/ui/02-ui-design-specification.md` | Approved design specification (faithful to the Phase 0 inventory) | Primary design input; reconciled below where gaps exist |
| `docs/ui/03-operational-dashboard-design.md` | **Design exploration** (not approved as MVP scope) | Evaluated element-by-element; elements either absorbed, simplified, or deferred per conflict resolutions below |

Where doc 03 conflicts with the Phase 0 baseline, the baseline governs unless the Board explicitly amends it (each amendment is a resolved conflict below, with documents requiring updates identified).

Doc 03 renumbers screens (S-01..S-21) differently from the authoritative inventory (S-01..S-15). **The authoritative numbering from `docs/ui/00-screen-inventory.md` is binding.** Doc 03's numbering is an exploration artifact and is not used in any reconciliation output.

---

## C-01: Competing Overview designs — rankings-first vs. operational dashboard

**UI expectation (doc 03 §1):** The Overview is an operational dashboard: observed-conditions panel, forecast consensus panel, multi-provider temperature chart, provider reliability list, forecast-changes feed, operational-health strip — with rankings compressed into a "Provider Reliability" widget.

**Backend/product capability:** The Phase 0 inventory S-01 is a rankings screen: ranking table + quick stats + methodology footer, served by `GET /rankings` + `GET /locations`. The Product Contract NP-01 explicitly non-promises "real-time weather (what is the weather now)": *"We measure forecasts, we don't deliver weather."* The product vision's differentiator is transparent, evidence-based comparison of forecasts against observed reality — not a current-conditions dashboard.

**Mismatch:** Doc 03's Overview reframes the product as a weather dashboard with rankings as one widget. Several of its elements (consensus, changes feed, confidence rating, disagreement index) have no defined backend capability or methodology. The operational-health strip exposes internal operational metrics on a public screen, contradicting doc 02 §2.2 principle 4 ("Admin is separate but accessible; never mixed into public data views").

**Options:**
1. Adopt doc 03's Overview wholesale (requires 4+ new backend capabilities + methodology extensions).
2. Keep doc 02's rankings-first Overview unchanged; defer all doc 03 elements.
3. Keep rankings-first Overview; absorb only elements that (a) map to existing data, (b) require no new methodology, and (c) do not drift toward a consumer weather app.

**Tradeoffs:**
- UX: option 1 is visually richer but misrepresents the product's first-day value (a new location has no rankings for 7+ days; a current-conditions dashboard would mask the platform's actual state). Options 2–3 are honest about what the platform does.
- Engineering: option 1 requires consensus aggregation, change detection, confidence derivation, disagreement index — four new computation paths with no methodology version, violating "no unexplained metrics." Options 2–3 stay within the approved API.
- Data: option 1's "Confidence: Moderate" has no documented calculation → violates reconciliation principle 3.
- Product truth (principle 7): option 1 drifts toward a generic consumer weather application.

**Decision:** Option 3. The approved Overview is doc 02 §4 (rankings-first). Absorbed from doc 03:
- Location Context Bar's "last observation" timestamp + provenance badge — supported by existing `observations` data (latest observation per location is one indexed query; provenance is stored). This is **evidence context for rankings**, not a weather display. Added to S-01 as a single context line (not a conditions panel).
- Provider Reliability list styling (rank + score + sample context) — already the ranking table.
- Operational-health strip: **removed from public Overview** (admin has S-10). Freshness indicator retained (already specified, BR-FRESH).

Deferred to Commercial Beta (Level 3): forecast consensus panel, forecast-changes feed, confidence rating, disagreement index, observed-conditions panel, multi-provider live chart as primary content.

**Documents requiring updates:** `docs/ui/00-screen-inventory.md` (S-01 context line), `docs/ui/04-approved-information-architecture.md` (new), `docs/ui/05-screen-specifications.md` (new), `docs/planning/03-ui-mvp-classification.md` (new).
**Owner:** Product Manager. **Status:** RESOLVED.

---

## C-02: Forecast Changes feed (DR-06) — no backend capability

**UI expectation (doc 03 §1.1, Appendix A):** A "Forecast Changes" feed listing material deltas between successive forecast issuances ("Rain probability 44% → 72%, High impact, 14:32 MYT").

**Backend capability:** None. The domain model stores immutable snapshots; no diff computation, no change-detection service, no "impact level" classification exists anywhere in the methodology or requirements.

**Mismatch:** Entirely new backend capability + invented classification ("High/Medium/Low impact" has no definition).

**Options:**
1. Build for MVP: window-function diff over snapshots + impact thresholds.
2. Defer to Level 3 with a reserved data-model note.
3. Remove permanently.

**Tradeoffs:**
- Engineering: the diff query is feasible (`LAG()` over `(provider_id, location_id, target_time)` ordered by `issued_at`) but impact classification requires product-defined thresholds that do not exist — inventing them violates "no invented data."
- Value: high for the eventual product story ("when did the forecast shift?") but not required to prove the core value (accuracy comparison).
- MVP simplicity (principle 5): defer.

**Decision:** Deferred to Commercial Beta. No MVP navigation item, no route, no API. **Reserved now:** the snapshot schema already supports the query (indexed `(provider_id, location_id, target_time, issued_at)` access path via existing composite indexes); no schema change required. Impact-level taxonomy must be defined in a future methodology extension before implementation.

**Documents:** `docs/planning/03-ui-mvp-classification.md`, `docs/domain/04-ui-domain-model-reconciliation.md` (reservation note).
**Owner:** Principal Software Architect. **Status:** RESOLVED (deferred).

---

## C-03: Forecast Consensus + Confidence Rating (DR-07, DR-08) — undefined methodology

**UI expectation (doc 03):** "FORECAST CONSENSUS" panel: predicted temperature (single value), rain probability (single value), expected rainfall, "Confidence: Moderate," "Provider agreement 3/4."

**Backend capability:** Snapshots for all providers at a target time are stored and queryable. A mean/median aggregation is mechanically possible. However:
- No methodology document defines "consensus" (mean? median? weighted by provider accuracy?).
- "Confidence: Moderate" combines CI width and provider disagreement per doc 03 Appendix B — **no formula exists**. Publishing it violates principle 3 (no unexplained metrics) and PC-02 (every published number shows how it was computed).
- "Provider agreement 3/4" requires a threshold for "agreeing" (within ±X°? ±Y mm?) — undefined.

**Mismatch:** Values are derivable in principle but lack documented calculation → cannot be published under the product contract.

**Options:**
1. Define consensus + confidence methodology now and build for MVP.
2. Defer both; reserve nothing (pure query, no schema impact).
3. Publish agreement count only (needs only a threshold definition).

**Tradeoffs:**
- Statistical correctness (principle 6): a naive mean across providers of differing skill is misleading; a skill-weighted consensus is a research-grade methodology extension — inappropriate for MVP scope.
- MVP simplicity: defer.

**Decision:** Deferred to Commercial Beta, gated on a methodology extension (new section in `docs/domain/03-metric-methodology.md` defining aggregation rule, agreement threshold, and confidence formula with a new methodology_version). No schema preparation required (pure query over existing snapshots). MVP UI must not display any consensus/confidence value.

**Documents:** `docs/planning/03-ui-mvp-classification.md`, `docs/domain/05-metric-ui-contract.md` (exclusion note).
**Owner:** Principal Data Engineer + Product Manager. **Status:** RESOLVED (deferred).

---

## C-04: Observed-conditions panel and "Live Weather" screen (NP-01 tension)

**UI expectation (doc 03 §1.1, §9.1 S-02, §10.4):** An "OBSERVED" panel (temperature, feels-like, rainfall, humidity, wind, pressure, condition) and a dedicated Live Weather screen with map.

**Backend capability:** Observations are stored with full provenance. `GET /observations` exists (user+). Latest-observation queries are cheap (indexed).

**Mismatch:** Product Contract NP-01: "Real-time weather — We measure forecasts, we don't deliver weather." The Phase 0 inventory contains no Live Weather screen. Doc 03 itself acknowledges the tension (Appendix B: "Framed as 'what our observation source reports' with full provenance; not a consumer weather display").

**Options:**
1. Add Live Weather screen + conditions panel to MVP.
2. Remove entirely; observations appear only as provenance context on comparison screens.
3. Expose latest observation as compact context (one line: value + time + provenance badge) on data screens, without a dedicated screen or multi-variable conditions panel.

**Tradeoffs:**
- Product truth (principle 7): a full conditions panel + map screen drifts toward consumer weather. A one-line context ("Last observation: 31.4 °C at 18:05 MYT [Reanalysis]") supports the user's trust in the ground-truth source without becoming a weather app.
- "Feels like" (DR-09): heat-index derivation from observations is not stored and would be a new derived value without methodology → excluded.
- Map (doc 03 §10.4): adds external tile dependency (CartoDB), deferred with heatmap per MVP scope §5.

**Decision:** Option 3. No Live Weather screen in MVP. Latest observation (temperature + precipitation status + timestamp + provenance badge) appears as a context line on S-01 and S-02. Full observation history remains available via API (`GET /observations`, user+) and is displayed on S-05 (observation line with provenance). Map deferred to Level 3. "Feels like" excluded from MVP UI.

**Documents:** `docs/ui/00-screen-inventory.md` (context-line note), `docs/ui/05-screen-specifications.md`, `docs/planning/03-ui-mvp-classification.md`.
**Owner:** Principal Product Designer + Product Manager. **Status:** RESOLVED.

---

## C-05: Alerts and Reports navigation items in MVP UI

**UI expectation (doc 03 §1.3):** Sidebar includes "Alerts" and "Reports" nav items; doc 03 §9.2 states the Alerts screen shows "Alerts available in a future release" empty state in MVP; a notification bell with count badge appears in the top bar.

**Backend/product capability:** Alerts are Level 3 (NP-03, MVP scope §5, domain model: ALERT_RULE removed). No alert entities, no notification infrastructure, no reports engine in MVP.

**Mismatch:** The reconciliation board mandate is explicit: *"If alerts are deferred, ensure the UI is removed from MVP navigation rather than displayed as non-functional."* Doc 03's approach (nav item → dead-end empty state; bell with badge count implying notifications exist) violates this and misleads users about capability.

**Options:**
1. Keep nav items with "coming soon" empty states (doc 03's approach).
2. Remove Alerts, Reports, and the notification bell entirely from MVP navigation; reserve layout space only.

**Tradeoffs:**
- UX honesty: option 2. Dead nav items are broken promises; the product contract NP-03 exists precisely to prevent this.
- Engineering: zero cost either way; option 2 is simpler.
- Doc 02 (approved spec) already handles this correctly (§9.2: "No nav item, no route, no screen in MVP. The header layout accommodates a future bell icon without reflow").

**Decision:** Option 2. Remove Alerts, Reports, Integrations, and the notification bell from all MVP navigation. The reserved-pattern documentation in doc 02 §9 stands. The event seam (`provider.health_changed` etc.) remains for Level 3.

**Documents:** `docs/ui/04-approved-information-architecture.md`, `docs/ui/00-screen-inventory.md` (confirmation note).
**Owner:** Principal UX Architect. **Status:** RESOLVED.

---

## C-06: Navigation model — header nav vs. sidebar

**UI expectation:** Doc 00 §1 and doc 02 §3 specify a single-row global header nav (Overview · Trends · Methodology · Admin · Settings). Doc 03 §1.3 specifies a 240px persistent sidebar with grouped sections (Overview, Data Operations, Administration) and 16 items.

**Backend capability:** Navigation is frontend-only; no backend impact. The question is which model serves the **approved MVP screen set** (15 screens, of which 5 are admin).

**Mismatch:** Doc 03's sidebar exists to accommodate screens that are not in MVP (Live Weather, Forecasts, Alerts, Reports, Observations, Integrations, Team as separate item). With those removed, the sidebar's "Data Operations" group collapses to Collection Health (one item) and its primary group to five items.

**Options:**
1. Header nav (doc 02) — matches authoritative inventory §1.
2. Sidebar (doc 03) — accommodates future growth.

**Tradeoffs:**
- Consistency with authoritative docs: header nav.
- Future growth: sidebar is easier to extend, but adopting it now for 7 public items + 5 admin items is premature; the admin sub-nav tab pattern (doc 02 §3.5) already solves the admin grouping.
- Risk: low either way; frontend-only.

**Decision:** Header nav per doc 02 §3 for MVP (authoritative inventory §1 governs). The sidebar design is retained as the documented Level 3 evolution path if screen count grows beyond ~12 top-level items. Admin sub-navigation remains the tab bar per doc 02 §3.5.

**Documents:** `docs/ui/04-approved-information-architecture.md`.
**Owner:** Principal UX Architect. **Status:** RESOLVED.

---

## C-07: Provider count in design artifacts — 4 shown, 2 approved

**UI expectation (doc 03):** Charts, legends, and mock data show 4 providers (Open-Meteo, Tomorrow.io, Visual Crossing, OpenWeather).

**Backend/product capability:** MVP providers are Open-Meteo + OpenWeather only (MVP scope §2.2; Tomorrow.io is the documented fallback; Visual Crossing is Level 3).

**Mismatch:** Design artifacts show capacity that does not exist in MVP. This is acceptable as *capacity demonstration* in design docs but must not leak into MVP acceptance criteria or imply backend support.

**Options:**
1. Redraw all designs with 2 providers.
2. Keep 4-provider designs as "capacity illustrations" with an explicit note; MVP renders 2.

**Tradeoffs:** Option 2 preserves design intent and the reserved provider color slots (doc 02 §1.3 already reserves colors 3–4) at zero cost. The backend is provider-agnostic (adapter interface), so 4-provider rendering works automatically when providers are added.

**Decision:** Option 2. All reconciliation documents specify behavior for N providers (MVP N=2). Provider colors for providers 3–4 remain reserved. No backend change.

**Documents:** `docs/ui/05-screen-specifications.md` (note).
**Owner:** Principal Frontend Engineer. **Status:** RESOLVED.

---

## C-08: S-03 Provider Detail requires a public collection-health subset, but `/forecast-collections` is admin-only

**UI expectation (doc 00 §4, doc 02 §7):** S-03 shows "Collection reliability vs. coverage" and a collection window (first/last snapshot timestamps per provider-location). Doc 00 maps S-03 to `/forecast-collections (public health subset)`.

**Backend capability:** `GET /forecast-collections` is **admin-only** (API requirements §4.5). Coverage and reliability values already appear in public `/rankings` and `/accuracy/summary` responses (component scores). First/last snapshot timestamps are not exposed publicly anywhere.

**Mismatch:** The screen inventory's API mapping references a "public subset" that was never defined in the API requirements. Exposing raw collection records publicly would leak operational details (error messages, latencies, payload keys) — security review objects.

**Options:**
1. Define a formal public subset endpoint `GET /providers/{id}/collection-summary`.
2. Extend the existing public `/accuracy/summary` response with a `collection_window` block (first_snapshot_at, last_snapshot_at, coverage, reliability per provider-location).
3. Drop the collection window from S-03/S-02.

**Tradeoffs:**
- Security: options 1–2 expose only derived aggregates, not raw operational records. Option 2 is additive within v1 governance (new optional fields).
- Coupling: option 2 couples window data to the accuracy payload — acceptable because coverage/reliability are already there; the window timestamps are two additional fields from one indexed MIN/MAX query.
- Frontend complexity: option 2 avoids an extra request.

**Decision:** Option 2. `GET /accuracy/summary` gains `collection_window: {first_snapshot_at, last_snapshot_at}` per provider entry (additive, v1-compatible). `/forecast-collections` remains admin-only. Doc 00's S-03 API mapping is corrected to `/accuracy` + `/accuracy/summary` + `/rankings`.

**Documents:** `docs/api/01-screen-api-contracts.md`, `docs/ui/00-screen-inventory.md` (mapping correction), `docs/ui/08-ui-backend-traceability.md`.
**Owner:** API Architect. **Status:** RESOLVED.

---

## C-09: Admin Users screen (S-14) references endpoints that do not exist

**UI expectation (doc 02 §11.4):** User list with disable/delete actions, admin-triggered GDPR export, audit log.

**Backend capability:** API requirements §4.5 defines only `GET /admin/audit-events`. No admin user-management endpoints exist (`GET /admin/users`, disable, delete, admin-triggered export). AUTH-09 and ADMIN-05 require these capabilities but the endpoint inventory omitted them.

**Mismatch:** Missing API capabilities for an MVP screen (S-14 is in the authoritative inventory).

**Options:**
1. Add endpoints: `GET /admin/users`, `PATCH /admin/users/{id}/status`, `DELETE /admin/users/{id}`, `POST /admin/users/{id}/export`.
2. Reduce S-14 to audit-log-only; manage users via Supabase dashboard.

**Tradeoffs:**
- Requirements coverage: ADMIN-05 is High priority; option 2 fails the requirement.
- Security: admin user management requires role check + audit logging + self-lockout guard (doc 02 §11.4 rule). Supabase admin API calls are server-side (service-role key never exposed to browser).
- Engineering: four straightforward CRUD endpoints in the identity module.

**Decision:** Option 1. Add the four endpoints to the API contract (see `docs/api/01-screen-api-contracts.md` §8). Account disable/delete propagate to Supabase Auth via server-side admin API (service-role key, backend only). Self-lockout guard: admin cannot disable/delete own account (409).

**Documents:** `docs/api/01-screen-api-contracts.md`, `docs/api/00-api-requirements.md` (amendment), `docs/security/01-ui-authorization-matrix.md`.
**Owner:** Principal Backend Engineer. **Status:** RESOLVED.

---

## C-10: Admin Health "Retry failed slot" conflates replay with re-collection

**UI expectation (doc 02 §10.3):** Expanded health row offers "[Retry failed slot]" mapped to `POST /admin/collections/{id}/replay`, disabled while circuit is open.

**Backend capability:** Replay (FC-14, domain model §4.8) reprocesses a **stored raw payload** through the current adapter. A failed collection (HTTP error, timeout, rate limit) typically has **no payload** — there is nothing to replay. The recovery action for a failed collection is a **new collection attempt** for that provider-location.

**Mismatch:** The UI's retry action targets the wrong backend operation. Replay is the correct action for *adapter-bug recovery on stored payloads* (S-13); the health screen needs *trigger-immediate-collection*.

**Options:**
1. Add `POST /admin/collections/trigger {provider_id, location_id}` — immediate out-of-band collection respecting rate limits and circuit state.
2. Reuse replay semantics (wrong: no payload exists for failures).

**Tradeoffs:**
- Correctness: option 1 is the operationally correct action (runbook: "circuit half-open → probe" is automatic; manual re-collection is the operator's lever for rate-limit recovery).
- Safety: the trigger endpoint must respect the provider's token bucket and must not fire while the circuit is open (409 with explanation) — matching the UI's disabled-button rule.
- Idempotency: a triggered collection that succeeds is naturally deduplicated by the snapshot uniqueness constraint if the scheduler fires again.

**Decision:** Option 1. Add `POST /admin/collections/trigger` (admin, audited, rate-limit-respecting, 409 while circuit open). S-10's button is relabeled "Re-collect now" and maps to this endpoint. Replay remains on S-13 for stored payloads.

**Documents:** `docs/api/01-screen-api-contracts.md`, `docs/api/00-api-requirements.md` (amendment), `docs/ui/05-screen-specifications.md` (S-10).
**Owner:** Principal Backend Engineer + Principal SRE. **Status:** RESOLVED.

---

## C-11: Admin Health system-level data (payload volume, engine lag, backup status) has no defined source

**UI expectation (doc 02 §10.2):** System Health panel: payload volume usage %, last backup time + status, last restore-test time + status, engine lag.

**Backend capability:** `GET /admin/health` is specified as "per provider-location: last success, circuit state, error counts, freshness" — no system-level section. Payload volume is a filesystem stat on the VPS; engine lag is derivable (`now() − max(accuracy_metrics.calculated_at)`); backup/restore results are produced by external scripts (cron + pg_dump), not the application.

**Mismatch:** Three of four values exist outside the application's current data surface.

**Options:**
1. Extend `/admin/health` with a `system` section; backup scripts write a status file (`/var/lib/forecastiq/backup-status.json`) that the app reads; payload volume via `statfs`; engine lag via DB query.
2. Drop the System Health panel; operators use Grafana/SSH.
3. Require Prometheus queries from the frontend (violates "do not require querying logs/metrics for normal UI behavior").

**Tradeoffs:**
- Operational usability (Journey J4 daily health loop): option 1 gives the operator one pane of glass. All four values are cheap reads (no new tables; one status file convention).
- MVP simplicity: option 1 adds ~1 endpoint section + a file-reading helper. Acceptable.
- Option 3 violates the reconciliation principle on log/metric querying.

**Decision:** Option 1. `GET /admin/health` response gains:
```
system: {
  payload_volume: {used_bytes, total_bytes, used_pct},
  engine_lag_seconds,
  last_backup: {completed_at, status},
  last_restore_test: {completed_at, status}
}
```
Backup scripts write the status file (runbook requirement, `docs/operations/01-ui-operational-signals.md`). Observation-collector status (doc 02 §10.2) is served from the same endpoint using `observations` max timestamps per location.

**Documents:** `docs/api/01-screen-api-contracts.md`, `docs/operations/01-ui-operational-signals.md`, `docs/ui/05-screen-specifications.md` (S-10).
**Owner:** Principal SRE. **Status:** RESOLVED.

---

## C-12: S-13 "next scheduled run" has no API field

**UI expectation (reconciliation board mandate + doc 00 S-13):** Schedule visibility including next scheduled run per provider-configuration.

**Backend capability:** `provider_configurations.collection_schedule` (JSONB interval) exists; `forecast_collections.requested_at` history exists. Next run = last slot time + interval (computed). No endpoint exposes it.

**Mismatch:** Minor gap — the value is derivable but not served.

**Options:**
1. Add `next_run_at` to a new `GET /admin/schedules` response (or extend provider-configurations exposure via an admin schedules endpoint).
2. Compute client-side from schedule interval + last collection time (fragile: client must know slot-claim semantics).

**Decision:** Option 1. `GET /admin/health` per-cell response includes `next_scheduled_at` (server-computed from schedule + last claim). No separate schedules endpoint needed for MVP — the health grid is the schedule-status surface; schedule *editing* remains on S-11 via `PUT /admin/provider-configurations/{id}`.

**Documents:** `docs/api/01-screen-api-contracts.md`.
**Owner:** Principal Backend Engineer. **Status:** RESOLVED.

---

## C-13: Operational dashboard's "disagreement" and chart elements vs. Forecast Evolution scope

**UI expectation (doc 03 §10.2):** A "Forecast Evolution" design proposal: issuance-time axis chart, truth zone band, disagreement index chart, change log panel, brush selection.

**Backend capability:** The data supports the core query (all predictions by a provider for one target time ordered by issued_at — snapshots indexed on `(provider_id, location_id, target_time)`; `issued_at` available). The disagreement index and change log are new computations (see C-02, C-03).

**Mismatch:** The authoritative inventory's S-05 (Forecast vs. Actual) is a *target-time-axis* chart (forecasts for one day vs. observation). Doc 03's Forecast Evolution is an *issuance-time-axis* chart — a genuinely different screen answering "how did predictions for one target evolve?" This is valuable and supportable but is **not** in the Phase 0 inventory.

**Options:**
1. Add issuance-axis Forecast Evolution to MVP (new screen + query, no new entities).
2. Defer to Level 3; MVP's S-05 covers the "forecast vs. reality" question; the issuance-axis question is documented as reserved.
3. Fold a minimal version into S-05 (issuance selector showing one issuance's track).

**Tradeoffs:**
- MVP scope discipline: the Phase 0 inventory was amended and frozen; adding a screen post-freeze requires justification. The core value proof (accuracy comparison) does not depend on it.
- Engineering: the query is cheap and indexed; the screen is mostly frontend work. Tempting, but scope creep at reconciliation is precisely what the board must prevent.
- Doc 02's DR-02 already resolves issuance selection for S-05 (horizon-matching issuance), which covers the most common user need.

**Decision:** Option 2. Issuance-axis Forecast Evolution deferred to Commercial Beta. The required query pattern and index sufficiency are documented in `docs/data/01-query-and-index-requirements.md` §3 so Level 3 implementation is additive. MVP S-05 remains as specified in doc 02 §5 with DR-02's issuance rule.

**Documents:** `docs/planning/03-ui-mvp-classification.md`, `docs/data/01-query-and-index-requirements.md`.
**Owner:** Principal Software Architect. **Status:** RESOLVED (deferred).

---

## C-14: Command Palette (⌘K), drawer pattern, and shell chrome from doc 03

**UI expectation (doc 03 §1.4, §5.1):** ⌘K command palette for search/navigation; provider-detail slide-over drawer; sidebar collapse behavior.

**Backend capability:** A command palette searching locations/providers/screens needs `GET /locations` + `GET /providers` (both public, existing). No new backend capability.

**Mismatch:** None technically — the question is MVP necessity. With 15 screens and flat header nav, search-based navigation adds little value; it is polish, not capability.

**Options:**
1. Include command palette in MVP (frontend-only, ~2 days).
2. Defer; keep direct navigation.

**Tradeoffs:** Pure frontend cost/benefit. MVP priority is state completeness (DB-02), not navigation polish. The drawer pattern for provider detail is likewise optional (S-03 exists as a full page).

**Decision:** Deferred to Commercial Beta (polish items). MVP uses direct navigation; provider detail is a full page (doc 02 §7). No backend impact.

**Documents:** `docs/planning/03-ui-mvp-classification.md`.
**Owner:** Principal Frontend Engineer. **Status:** RESOLVED (deferred).

---

## C-15: Doc 03 top-bar "API p50: 340ms" and retry counts on public screens

**UI expectation (doc 03 §1.1 operational strip):** "API p50: 340ms · Retries: 0" visible on the public Overview.

**Backend capability:** API latency percentiles exist only in Prometheus `/metrics` (operator-facing). Retry counts are collection-internal.

**Mismatch:** Exposing internal latency metrics and retry counts publicly: (a) mixes admin concerns into public views (doc 02 §2.2.4), (b) has no API surface by design (metrics are scraped, not served through the REST API), (c) provides no decision value to the public persona (Daniel does not act on p50 latency).

**Decision:** Removed. Operational metrics remain in Grafana + `/metrics` + admin S-10. Public screens show only freshness states (BR-FRESH) — the user-facing trustworthiness signal.

**Documents:** `docs/ui/06-ui-state-contracts.md`, `docs/operations/01-ui-operational-signals.md`.
**Owner:** Principal SRE. **Status:** RESOLVED.

---

## C-16: Timezone/date context element — doc 03 shows a date picker in the top bar

**UI expectation (doc 03 §1.4):** Top bar shows "Mon, Jul 21, 2026 · MYT (UTC+8)" as an interactive date context.

**Backend capability:** None needed (display of location timezone from `locations.timezone`; BR-TZ rules).

**Mismatch:** Minor: doc 02's header shows location + horizon selectors only; date context appears per-screen where relevant (S-05 date picker, S-04 period selector). A global date in the top bar implies global date filtering that the rankings screen does not use (rankings use evaluation *period*, not date).

**Decision:** No global date control. Timezone label appears in the Location Context Bar per screen (doc 02 pattern, BR-TZ-04 zone labels). Date selection remains per-screen (S-04, S-05). Doc 03's top-bar date is removed.

**Documents:** `docs/ui/04-approved-information-architecture.md`.
**Owner:** Principal UX Architect. **Status:** RESOLVED.

---

## C-17: Onboarding (S-07) — Supabase-hosted vs. app-defined flow boundaries

**UI expectation (doc 00 S-07, S-08):** Auth pages (sign in / sign up / verify / reset) + first-use onboarding explaining what the platform measures.

**Backend capability:** Supabase Auth manages credential flows (ADR-008). The app owns: user-row creation on first login (auth_subject mapping), onboarding content, dismissal persistence (DR-04: localStorage).

**Mismatch:** Doc 02 §14.8 open item 2 asks whether to use Supabase pre-built auth UI or custom forms. The board resolves: **custom forms calling the Supabase JS SDK** — the auth pages must match the design system and accessibility standards (WCAG AA focus states, labels, error announcements); hosted pages cannot be styled to the approved design system. Password screens are Supabase-SDK-backed (no app-managed password handling — consistent with ADR-008; the app never touches credentials).

**Decision:** Custom auth forms + Supabase SDK. Email verification and password reset are Supabase-hosted confirmation emails with app-origin links. Onboarding (S-07) is app content; dismissal in localStorage keyed by user ID (DR-04 approved). No new backend endpoints.

**Documents:** `docs/ui/05-screen-specifications.md` (S-07, S-08), `docs/api/01-screen-api-contracts.md` (auth boundary note).
**Owner:** Principal Frontend Engineer. **Status:** RESOLVED.

---

## C-18: Export — client-side CSV vs. server-side generation

**UI expectation (doc 02 §14.7, DR-05):** "Export CSV" on S-01/S-02/S-04/S-05 generated client-side from current view data with `#`-prefixed metadata header rows.

**Backend capability:** All exported data is already in the client's memory (fetched for display). Views are bounded (ranking table ≤ provider count; trends ≤ 365 daily points; FvA ≤ 24 hourly points × providers). GDPR export (`POST /me/export`) is separately server-side (AUTH-09).

**Mismatch:** None for MVP — client-side export is bounded, synchronous, and carries the same provenance the user sees (PC-09). Risk: client-side export could omit data the user filtered away (acceptable: "exports current filters" is the specified behavior, doc 00 §3).

**Decision:** Approved as designed. DR-05's metadata header format is ratified as the binding CSV contract (documented in `docs/api/02-response-conventions.md` §5). Server-side report generation remains Level 3. Row limits: trends exports capped at 365 rows (max period), FvA at 24×providers rows — bounded and predictable per the board's export guidance.

**Documents:** `docs/api/02-response-conventions.md`, `docs/ui/05-screen-specifications.md`.
**Owner:** Principal Frontend Engineer + QA Lead. **Status:** RESOLVED.

---

## C-19: Public read on raw forecasts/observations vs. AUTH-08's "user+" gating

**UI expectation (doc 02 §14.1):** S-05 (Forecast vs. Actual) is a **public** screen per the authoritative inventory, but its data comes from `GET /forecasts` and `GET /observations`, which API requirements §4.2 gate at **user+**.

**Backend capability:** AUTH-08: "Public read access: rankings/accuracy/providers/locations readable without auth; forecasts/observations raw queries and admin require auth."

**Mismatch:** A public screen depends on user-gated endpoints. Unauthenticated users cannot load S-05's chart.

**Options:**
1. Make S-05 require sign-in (change screen inventory auth column).
2. Add a public, bounded, derived endpoint for the FvA view: `GET /accuracy/comparison` returning snapshots + observations for one location/date/variable with attribution (no raw-query flexibility).
3. Make `GET /forecasts`/`GET /observations` public.

**Tradeoffs:**
- Product intent: the portfolio MVP is meant to be *publicly demonstrable* — gating the most visually compelling screen behind registration hurts the portfolio goal.
- Security/data: option 3 exposes raw queryable data (bulk-scrapeable) — the ToS review (D-05, BR-LIC-01) has not cleared redistribution; option 2 exposes only what the screen renders (bounded, attributed, rate-limited by IP).
- Engineering: option 2 is one read endpoint joining two indexed queries (~24+24 rows).

**Decision:** Option 2. New public endpoint `GET /forecast-comparison?location_id=&date=&variable=&providers=` returns the bounded FvA payload (snapshots per provider for the date at the selected horizon-matching issuance, observations with provenance, day metrics per provider, attribution, freshness). Raw `/forecasts` and `/observations` remain user+ per AUTH-08. S-05 remains public.

**Documents:** `docs/api/01-screen-api-contracts.md`, `docs/api/00-api-requirements.md` (amendment), `docs/ui/00-screen-inventory.md` (S-05 API mapping correction).
**Owner:** API Architect + Security Architect. **Status:** RESOLVED.

---

## C-20: Doc 03 mock data as design fixture vs. "no invented data"

**UI expectation (doc 03 Appendix A):** Detailed mock data (Johor Bahru, 21 July 2026) used to illustrate the design.

**Mismatch risk:** Mock data in design documents can leak into acceptance criteria or be mistaken for real provider output.

**Decision:** Doc 03's appendix is ratified as a **design fixture only** (clearly labeled). No reconciliation output reproduces it as expected system output. All acceptance criteria reference test vectors from the methodology document (§10), not design mock data.

**Documents:** None requiring change (note in this register).
**Owner:** QA Lead. **Status:** RESOLVED.

---

## Summary

| Conflict | Severity | Disposition | Blocking? |
|----------|----------|-------------|-----------|
| C-01 Competing Overview designs | Critical | Rankings-first retained; context line absorbed | No (resolved) |
| C-02 Forecast Changes feed | High | Deferred Level 3 | No |
| C-03 Consensus + Confidence | High | Deferred Level 3 (methodology gate) | No |
| C-04 Live Weather / conditions panel | High | Removed; context line only | No |
| C-05 Alerts/Reports nav items | High | Removed from MVP navigation | No |
| C-06 Header vs. sidebar nav | Medium | Header nav (authoritative) | No |
| C-07 Provider count in designs | Low | Capacity illustration; MVP N=2 | No |
| C-08 Public collection-health subset | High | `collection_window` added to `/accuracy/summary` | No |
| C-09 Missing admin user endpoints | Critical | Four endpoints added | No |
| C-10 Retry vs. re-collect | High | `POST /admin/collections/trigger` added | No |
| C-11 System health data sources | Medium | `/admin/health` system section + status file | No |
| C-12 Next scheduled run | Low | `next_scheduled_at` in health response | No |
| C-13 Forecast Evolution screen | Medium | Deferred Level 3; query documented | No |
| C-14 Command palette / drawer | Low | Deferred Level 3 | No |
| C-15 Operational metrics on public UI | Medium | Removed | No |
| C-16 Global date control | Low | Removed; per-screen dates | No |
| C-17 Auth UI approach | Medium | Custom forms + Supabase SDK | No |
| C-18 Export mechanism | Medium | Client-side CSV ratified (DR-05) | No |
| C-19 Public S-05 vs. user+ endpoints | Critical | New public `/forecast-comparison` endpoint | No |
| C-20 Mock data status | Low | Design fixture only | No |

**All 20 conflicts resolved. Zero blocking conflicts remain unowned.**
