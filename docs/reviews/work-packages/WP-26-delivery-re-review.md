# ForecastIQ — WP-26 Performance Validation (scaffold): DRB Confirmatory Re-Review

**Review date**: 2026-07-27
**Work package**: WP-26 — Performance and Reliability Validation, **slice 1 / scaffold** (PR #32, `feature/wp26-performance-validation`)
**Prior review**: WP-26-delivery-review.md — REJECTED on `cb5ce91` (DRB-WP26-001…007)
**Reviewed SHA**: `5115717`
**Decision**: **ACCEPTED at reduced scope (scaffold); remainder tracked as WP-26b**

---

## 1. Resolution taken

Option 2 of the original review: the package is accepted on its **honest reduced
scope** (k6 PT-1/PT-2/PT-6 + a request-path reliability slice + a
volume-estimating seeder), the concrete bugs are fixed, and everything not
delivered is tracked as **WP-26b** in the planning doc and the perf-doc baseline
register. WP-26's original "all PT scenarios + NFR-S01 evidence" objective is
explicitly **not** claimed complete.

## 2. Verification of evidence

| Check | Result |
|-------|--------|
| Commit identity: local branch == remote == CI head | ✅ all `5115717` |
| Six PR jobs green | ✅ first run |
| Seeder builds + vets; URL redaction + volume math verified by run | ✅ `postgres://forecastiq:xxxxx@…`; snapshots 1,497,600 ≈ 1.5M, matches ≈ 3M (doc §3) |
| Seeder exits non-zero without `--estimate-only` | ✅ (no silent empty-DB seeding) |
| `bash -n reliability.sh` | ✅ |

## 3. Finding closure

| Finding | Status | Resolution |
|---------|--------|-----------|
| 001 (H) reliability suite mislabeled as full | ✅ | Re-titled "slice 1 of N"; header + summary reference WP-26b; scope note lists the 5 absent fault-injection scenarios |
| 002 (H) missing PT scenarios / baselines untracked | ✅ Tracked | WP-26b added to planning doc (PT-3/4/7/8, fault injection, seeder writes, baselines, 2× load, scheduled wiring); perf §6 register annotated |
| 003 (M, security) DB credential leak | ✅ | `url.Redacted()`; verified password shows as `xxxxx` |
| 004 (M) volume math 100× off + exit 0 empty | ✅ | Fan-out corrected to doc §3 (~1.5M/3M); exits non-zero unless `--estimate-only` |
| 005 (M) rate-limit bucket flake | ✅ | Rate-limit scenario moved **last**; default-limiter requirement + k6-env mutual-exclusion documented in header |
| 006 (M) scheduled.yml stale contract | ✅ | Comment rewritten to point at WP-26b instead of "joins when WP-26 merges" |
| 007 (L) PT-1 error gate too loose | ✅ | `http_req_failed: rate==0` (doc §2 target) |

## 4. Verified correct (carried from first review)

Enforcing (non-decorative) k6 thresholds; env-driven versioned `/api/v1` URLs;
no ADR-033 topology drift (no systemctl/VPS); prior DRB shell remediations
present; deterministic seed.

## 5. Decision

**ACCEPTED (scaffold).** Six jobs green on `5115717`; all seven findings closed
or explicitly tracked; the credential leak (the one hard bug) is fixed and
verified. The package no longer over-claims — reliability.sh, the perf-doc
register, and the planning doc all state the reduced scope and point to WP-26b.
PR #32 ready to merge. **WP-26b** carries the completion work (PT-3/4/7/8, the
5 fault-injection scenarios, functional seeder, baselines, 2× load, scheduled
wiring). **WP-27 (Docs/Demo) becomes eligible.**
