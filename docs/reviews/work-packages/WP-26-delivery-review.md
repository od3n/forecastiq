# ForecastIQ — WP-26 Performance and Reliability Validation: Delivery Review Board

**Review date**: 2026-07-27
**Work package**: WP-26 — Performance and Reliability Validation (PR #32, `feature/wp26-performance-validation`)
**Reviewed SHA**: `cb5ce91` (post ADR-033 merge-up)
**Decision**: **REJECTED — acceptance bar not met (scaffold only); 1 concrete security bug**

---

## 1. Context

WP-26's objective is *"all PT scenarios green; reliability suite green; NFR-S01
evidence"* (baseline register populated, 2× volume load test). This branch is a
genuine, honestly-titled **scaffold**: what exists is real (k6 thresholds are
enforcing not decorative; scripts are env-driven and hit versioned `/api/v1`;
no `curl -f` or URL-versioning regressions; no ADR-033 topology drift — no
systemctl/VPS assumptions). But it delivers a fraction of the scope and cannot
produce the NFR-S01 evidence the WP exists to produce. Rejection is on
completeness + one must-fix bug, not on code quality of what's present.

## 2. Findings

### High

**DRB-WP26-001 (H)** — Reliability suite delivers ~2 of 7 mandated scenarios and
presents itself as the full suite. `reliability.sh` covers rate-limit and
malformed-payload (the latter degrades to a 401 auth-gate assertion without
`ADMIN_TOKEN`); the other checks (body size, 404, health, CORS) are WP-25
security assertions relabeled — no fault injection. **Absent**: provider
timeout, duplicate job, late observation, worker restart, DB reconnect — every
scenario needing real fault injection. Header claims "Reliability fault-injection
test suite (WP-26)" with no deferral note. Fix: implement the missing scenarios
(worker restart / DB reconnect via `docker compose restart app` /
`stop+start db` + readyz recovery assertion; timeout via a delay-injected fake
provider), or re-title as "slice 1 of N" with tracking.

**DRB-WP26-002 (H)** — 5 of 8 PT scenarios absent, baseline register untouched,
2× volume run absent, nothing tracks the gap. Present: PT-1, PT-2, PT-6.
Missing: PT-3 (ingestion burst), PT-4 (analysis batch, NFR-P06), PT-5
(evolution, Level-3 deferrable), **PT-7 (2× volume — the NFR-S01 Level-1 exit
gate)**, PT-8 (Lighthouse — no config anywhere). Doc §6 baseline register is
still the empty placeholder. Partial delivery is plan-consistent (slice plan
puts baselines in slice 3) but no in-repo note records the remainder, inviting
silent scope closure on merge.

### Medium

**DRB-WP26-003 (M, security)** — Seeder leaks DB credentials to stdout:
`fmt.Printf("DB: %s...", (*dbURL)[:min(40,len)])` prints the first 40 chars of
`postgres://user:password@host...`, which includes the password — and lands in
CI logs once wired into the scheduled workflow. Verified at `seeder/main.go:59`.
Fix: `url.Parse` + `u.Redacted()`.

**DRB-WP26-004 (M)** — Seeder volume math ~100× below doc §3 and exits 0 writing
nothing. `snapshots = loc×prov×days×24` = 14,400 for base vs doc's ~1.5M (omits
forecast-horizon × variable fan-out); the "Estimated volumes" banner is the
scaffold's documented interface, so an implementer following it seeds 100× too
little → false-green baselines. Program prints "scaffold complete" and exits 0
having written nothing, so `seeder && k6 run` gets no empty-DB signal. Fix:
correct formulas to §3; exit non-zero in scaffold mode (or require `--dry-run`).

**DRB-WP26-005 (M)** — Rate-limit reliability test drains the shared token bucket
(burst 120, refill 2/s); `sleep 2` refills ~4 tokens, so later tests can flake
with 429 instead of their expected 401/413/404 (RateLimit runs before auth).
Also a contradictory env contract: PT-1/PT-2 headers tell operators to raise
`FIQ_RATE_LIMIT_PER_IP_PER_MIN` to ~100000, under which the rate-limit scenario
can never see 429. The two suites can't share one env config, undocumented.
Fix: run the rate-limit scenario last with a proportional wait; document the
default-limiter requirement.

**DRB-WP26-006 (M)** — Not wired into `scheduled.yml`, whose own line 8 says the
weekly perf/reliability suites "join this file when WP-26 merges", and doc §2
marks PT-1/PT-6 weekly. Merging leaves that contract stated-but-unfulfilled.
Fix: add the weekly job (compose up → seeder → k6) or update the comment to the
tracked follow-on.

### Low

**DRB-WP26-007 (L)** — PT-1 gates `http_req_failed: rate<0.001` while doc §2 PT-1
target is "0 errors"; at ~48K iterations that admits ~48 failures through green.
Use `rate==0` or document the relaxation.

## 3. Verified correct

k6 thresholds are enforcing (k6 exits 99 on breach; PT-2 `iterations rate>=100`
valid); all URLs env-driven + versioned, every path exists in `router.go`;
`/healthz`/`/readyz` correctly unversioned; prior DRB remediations present
(curl status capture, file-based big body, grep tautology fix, PT-6 parse
safety); seeder builds/vets, deterministic (`--seed`, `rand.NewSource`), uses
correct `FIQ_DATABASE_URL`; **no ADR-033 topology drift** (no systemctl/VPS).

## 4. Scope coverage

3/8 PT scenarios (PT-1/2/6) · ~1.5/7 reliability scenarios · seeder
scaffold-only (writes nothing) · 0 baselines · no 2× volume run · no CI wiring.

## 5. Decision

**REJECTED.** DRB-WP26-003 (credential leak) is a must-fix regardless. The
package cannot be accepted as WP-26 "done" because its defining deliverables —
all PT scenarios, the reliability fault-injection suite, a populated baseline
register, and the NFR-S01 2× volume evidence — are largely absent, with no
in-repo tracking of the remainder.

Two acceptable resolutions (product decision):
1. **Complete WP-26**: implement the missing PT-3/4/7/8 + 5 reliability
   scenarios + functional seeder DB writes, run the 2× load test, populate the
   baseline register. Re-review requires green CI **plus** a rehearsed run
   producing real numbers.
2. **Formally descope to a scaffold WP**: fix 003–007, re-title reliability.sh
   and add a tracked follow-on WP (e.g. WP-26b) enumerating PT-3/4/7/8,
   reliability fault injection, seeder generation, baselines, and 2× load —
   recorded in doc §6 and the WP registry. Then this scaffold can be accepted
   on its honest, reduced scope.
