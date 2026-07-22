# ForecastIQ — UI MVP Classification

**Version**: 1.0 (UI ↔ Backend Reconciliation Board)
**Status**: Authoritative — binding for Phase 1 Architecture and frontend implementation
**Inputs**: Phase 0 Amendment baseline; `docs/reviews/03-ui-backend-conflicts.md`; `docs/planning/01-scope-levels.md`; `docs/product/03-mvp-scope.md`

Every UI capability identified during design exploration is classified below. Classifications follow the board's four categories: **Required** (necessary to prove core value), **Simplified** (included with reduced functionality), **Deferred** (valuable, unnecessary for first public version), **Removed** (does not support the product goal or cannot be justified).

Screen IDs use the authoritative numbering from `docs/ui/00-screen-inventory.md`.

---

## 1. Screens

| Capability | Classification | Rationale | Backend impact | UI impact | Migration path |
|------------|---------------|-----------|----------------|-----------|----------------|
| S-01 Overview (rankings-first) | **Required** | Core value front door: "which provider is best for my location + horizon?" | `GET /rankings`, `GET /locations` (existing) | Per doc 02 §4 + C-01 context line | — |
| S-01 latest-observation context line | **Simplified** | Trust in ground truth without becoming a weather app (C-04) | Latest-observation query (indexed; existing data) | One line: temp + precip status + time + provenance badge | Full conditions panel at Level 3 |
| S-02 Location Detail | **Required** | Per-variable deep comparison (PC-02 transparency) | `GET /accuracy/summary` (+ `collection_window` additive, C-08) | Per doc 02 §8 | — |
| S-03 Provider Detail | **Required** | Cross-location provider assessment; reliability vs. coverage distinction (PC-06) | `GET /accuracy`, `/rankings`; collection_window via `/accuracy/summary` | Per doc 02 §7 | — |
| S-04 Trends | **Required** | Accuracy-over-time = improving/degrading detection | `GET /accuracy?aggregation=` (existing) | Per doc 02 §6 | — |
| S-05 Forecast vs. Actual | **Required** | Most compelling evidence visualization; portfolio centerpiece | New public `GET /forecast-comparison` (C-19) | Per doc 02 §5 | — |
| S-06 Methodology | **Required** | PC-02, BR-RANK-06: methodology transparency is the differentiator | `GET /rankings/methodology` (existing) + static prose | Per doc 00 §2 S-06 | — |
| S-07 Onboarding | **Required** | First-use honesty: what we measure / don't measure (NP-01..07) | None (localStorage dismissal, DR-04) | Per doc 00 §3 | — |
| S-08 Auth pages | **Required** | Registration/verification/reset per AUTH-01 | Supabase Auth (managed); user-row mapping on first login | Custom forms + Supabase SDK (C-17) | — |
| S-09 Settings | **Required** | API keys (AUTH-05), GDPR export/delete (AUTH-09), preferences | `GET /me`, `/api-keys` CRUD, `/me/export`, `DELETE /me` (existing) | Per doc 02 §14.1 | — |
| S-10 Admin Health | **Required** | Operator's daily loop (J4); the MVP's "alert centre" | `GET /admin/health` extended (C-11, C-12); `POST /admin/collections/trigger` (C-10) | Per doc 02 §10, relabeled retry button | — |
| S-11 Admin Providers | **Required** | ADMIN-01 | Existing endpoints | Per doc 02 §11.1 | — |
| S-12 Admin Locations | **Required** | ADMIN-02, BR-LOC-01 | Existing endpoints (dedup 409) | Per doc 02 §11.2 | — |
| S-13 Admin Schedules & Runs | **Required** | ADMIN-03/04/06, FC-14 | Existing + trigger endpoint | Per doc 02 §11.3 | — |
| S-14 Admin Users & Audit | **Required** | ADMIN-05, AUTH-07/09 | Four new admin user endpoints (C-09) | Per doc 02 §11.4 | — |
| S-15 Error pages | **Required** | DB-02 state completeness | None (frontend + status codes) | 404, 500, 403, offline | — |
| Live Weather screen (doc 03 S-02) | **Removed** | NP-01: "we don't deliver weather"; no Phase 0 requirement | None | Not in nav; observation context lines only | — (observation API remains for Level 3 views) |
| Forecasts screen (doc 03 S-03, raw forecast detail) | **Deferred** | Data exists via user+ API; S-05 covers the comparison need | None new | No MVP screen | Level 3 screen over existing `/forecasts` |
| Issuance-axis Forecast Evolution (doc 03 §10.2) | **Deferred** | Valuable ("when did the forecast shift?") but not core-value proof; post-freeze scope addition | None now; query pattern documented (C-13) | No MVP screen | Level 3; indexes already sufficient |
| Alerts screen + nav (doc 03 S-07) | **Deferred** | NP-03; removed from MVP navigation per board mandate (C-05) | None; event seam preserved (ADR-006) | No nav item, no route, no bell | Level 3 with alert engine; layout reserves bell slot |
| Reports screen (doc 03 S-08) | **Deferred** | No report engine in MVP; CSV export covers PC-09 | None | No nav item | Level 3 |
| Integrations screen (doc 03 S-15) | **Deferred** | No webhook/external-consumer capability (MVP scope §5) | None | No nav item | Level 3 with webhooks |
| Live Weather Map (doc 03 §10.4) | **Deferred** | External tile dependency; 5–10 locations don't justify (MVP scope §5 heatmap deferral) | None | No MVP surface | Level 3 |
| Sidebar navigation (doc 03 §1.3) | **Deferred** | Header nav serves 15 screens; sidebar exists for removed screens (C-06) | None | Header nav per doc 02 §3 | Level 3 if top-level items exceed ~12 |
| Command palette ⌘K (doc 03 §5.1) | **Deferred** | Navigation polish; not capability | None (would use existing public endpoints) | Not in MVP | Level 3 |
| Provider slide-over drawer (doc 03 §5.2) | **Deferred** | S-03 full page exists | None | Full-page navigation | Level 3 polish |
| Notification bell + dropdown (doc 03 §1.4) | **Deferred** | No notifications in MVP (C-05) | None | Not rendered; layout slot reserved | Level 3 |

## 2. Overview Dashboard Elements (doc 03 reconciliation, per C-01)

| Element | Classification | Rationale | Backend impact |
|---------|---------------|-----------|----------------|
| Observed-conditions panel (multi-variable) | **Deferred** | NP-01 tension; context line suffices (C-04) | None now |
| Forecast consensus panel (DR-07) | **Deferred** | No methodology for consensus aggregation (C-03) | None now; pure query later |
| Confidence rating (DR-08) | **Deferred** | No documented formula → PC-02 violation if published (C-03) | Methodology extension required first |
| Provider agreement count | **Deferred** | Requires undefined "agreement" threshold (C-03) | None now |
| Forecast changes feed (DR-06) | **Deferred** | No change-detection capability; impact taxonomy undefined (C-02) | None now; LAG() query feasible later |
| Provider disagreement index (DR-10) | **Deferred** | New derived metric without methodology | None now |
| Multi-provider live temperature chart (as primary content) | **Deferred** | S-05 provides the comparison chart with evidence; dashboard primacy belongs to rankings | None now |
| Operational health strip (public) | **Removed** | Admin concerns don't belong on public views (C-15); S-10 is the operator surface | None |
| API p50 / retry counts (public) | **Removed** | Internal metrics; no REST surface by design (C-15) | None |
| Provider reliability list (rank + score + samples) | **Required** | = the ranking table (already specified) | Existing `/rankings` |
| Location context bar (name, tz, last observation, freshness, provenance) | **Required** (simplified) | BR-FRESH, BR-TZ, BR-OBS-01 | Latest-observation context line |
| Attribution footer | **Required** | BR-ATTR-01 | `attribution` in responses |

## 3. Global Controls and Chrome

| Capability | Classification | Rationale | Backend impact |
|------------|---------------|-----------|----------------|
| Location selector (URL-synced) | **Required** | doc 00 §1 | `GET /locations?active=true` |
| Horizon selector (7 horizons) | **Required** | CE-02 | `horizon_minutes` filter |
| Date range presets (7/30/90d + custom, max 365d) | **Required** | S-04 | `period_start/end` |
| Variable selector | **Required** | S-04/S-05 | `variable` filter |
| Timezone display toggle (BR-TZ-03) | **Required** | BR-TZ rules | None (client-side; `locations.timezone` in payload) |
| Global date control in top bar | **Removed** | Rankings use periods, not dates; per-screen dates suffice (C-16) | None |
| System status dot in sidebar footer (doc 03) | **Removed** | Sidebar deferred; `/healthz` exists for operators | None |
| Version indicator "v1.0.0" | **Simplified** | Optional footer text; no backend | None |

## 4. States and Behaviors

| Capability | Classification | Rationale |
|------------|---------------|-----------|
| All 10 mandatory states per screen inventory §3 | **Required** | DB-02, AC-6.1 |
| State priority ordering (doc 02 §13.8) | **Required** | Deterministic rendering |
| Offline banner + cached-data labeling | **Required** | PC-10 |
| Auto-refresh on Admin Health (60s polling, DR-03) | **Required** | Operational usability; cheap with ETag 304 |
| Chart zoom/pan/brush | **Deferred** | Doc 00 §5 (Level 3) |
| Dark mode | **Deferred** | Doc 00 §5 (Level 3) |
| Mobile-native layouts | **Deferred** | Responsive web suffices (MVP scope §5); responsive breakpoints per doc 02 §12 are **Required** |
| Heatmap | **Deferred** | MVP scope §5 |
| Radar overlay / wind barbs | **Deferred** | Level 3 extension of deferred map |
| Animated weather particles | **Removed** | Decorative; violates "numbers over decoration" |

## 5. UI-Discovered Requirements — Final Dispositions

| ID | Requirement | Disposition | Notes |
|----|-------------|-------------|-------|
| DR-01 | Quick-stats clickability | **No action** | Informational only; counts derivable client-side from `/rankings` |
| DR-02 | FvA issued-at selection rule | **Approved** | Horizon-matching issuance; subtitle states issued_at + horizon; no new endpoint |
| DR-03 | Health auto-refresh 60s | **Approved** | Polling + ETag; no new endpoint |
| DR-04 | Onboarding dismissal in localStorage | **Approved** | Client-side; no endpoint |
| DR-05 | CSV metadata header format | **Approved** | Binding format in `docs/api/02-response-conventions.md` §5 |
| DR-06 | Forecast change detection | **Deferred L3** | C-02 |
| DR-07 | Forecast consensus | **Deferred L3** | C-03; methodology gate |
| DR-08 | Confidence rating | **Deferred L3** | C-03; methodology gate |
| DR-09 | Real-time observation summary + feels-like | **Simplified** | Latest-observation context line approved; feels-like derivation excluded (not stored on observations) |
| DR-10 | Provider disagreement index | **Deferred L3** | C-03 |

## 6. Post-MVP Architectural Impact Classification

Per the board's mandate, deferred capabilities are classified by what (if anything) must be prepared now:

| Deferred capability | Architectural impact now |
|---------------------|--------------------------|
| Forecast changes feed (DR-06) | **No current impact** — snapshots + existing indexes suffice; LAG() query additive later |
| Consensus / disagreement (DR-07/10) | **No current impact** — pure queries over snapshots |
| Confidence rating (DR-08) | **No current impact** — methodology extension precedes implementation |
| Alerts (NP-03) | **Contract should be reserved now** — `workspace_id` schema reservation exists (domain model §8.2); event names/payloads preserved (ADR-006); bell-icon layout slot reserved. No further action. |
| Organization workspaces (NP-06) | **Data model preparation required now** — already done: `workspace_id` NOT NULL on ownership-bearing tables (domain model §8.2). No further action. |
| Live Weather / map | **No current impact** |
| Reports engine | **No current impact** — CSV export is client-side; server-side jobs are additive (`ExportJob` entity deferred, not reserved) |
| Additional providers (Tomorrow.io, Visual Crossing) | **No current impact** — adapter interface + reserved color slots exist |
| NOAA/NWS observations | **No current impact** — adapter slot documented (ADR-003) |
| Sidebar nav / command palette | **No current impact** — frontend-only |

**No architecture preparation beyond existing Phase 0 reservations is required.** This confirms the deferred set imposes zero cost on MVP engineering.
