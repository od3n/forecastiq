# ForecastIQ — CI/CD (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: NFR-M03..M07, NFR-SEC10; `docs/architecture/06-deployment-architecture.md` §4; deployment runbook

Tooling: **GitHub Actions** (constraints §2). Branch strategy: trunk-based on `main` with short-lived feature branches; release tags `vYYYY.MM.DD-N`.

---

## 1. Pull Request Pipeline

```yaml
jobs:
  backend-checks:
    - gofmt/goimports check
    - golangci-lint (incl. depguard module-boundary rules, govet, sqlclosecheck)  # NFR-M03
    - govulncheck                                                                 # NFR-SEC10
    - gitleaks (secret scan)
    - go test ./... (unit, race detector on)
    - property tests (gopter, 1000 cases/formula invariant)
    - coverage report (gate: analysis/domain ≥ 80%)
  backend-integration:
    - services: postgres:16
    - migrations up (verify all apply cleanly)
    - migration down/up reversibility check (reversible migrations)
    - go test -tags integration ./...
    - golden-path e2e (fake providers)
  api-contract:
    - generate OpenAPI from code
    - diff generated vs committed (drift gate)
    - openapi-diff vs main (breaking-change gate)                              # API §7
  frontend-checks:            # when web/ touched
    - npm ci + npm audit (high+ blocks)
    - eslint + tsc --noEmit
    - vitest (component + state-contract tests)
    - next build (static export succeeds)
    - axe-core suite (zero critical)                                           # a11y mandate
    - bundle checks: service-role key grep; budget 200 KB chart lib
  image:
    - docker build (distroless)
    - trivy image scan (critical blocks) + secret scan
```

All PR jobs blocking. Typical PR feedback < 10 min (integration parallelized).

## 2. Main Branch Pipeline (post-merge)

```yaml
jobs:
  build-release:
    - full test suite (including 10K-case property nightly-tier on schedule)
    - build binary (linux/amd64, CGO off, stripped) + checksums
    - dashboard static build
    - artifact upload (retained 90 d)
  deploy-dashboard:
    - Cloudflare Pages deploy (auto; zero downtime)
  deploy-api:                    # environment: production (manual approval gate)
    - migration dry-run against prod-schema copy
    - rsync artifact → VPS
    - migrate --confirm
    - systemd restart + readyz wait
    - smoke tests (healthz, readyz, rankings 200, admin login)
    - notify (webhook)
```

## 3. Scheduled Jobs

| Schedule | Job |
|----------|-----|
| Nightly | Extended property tests (10K cases); govulncheck full; Dependabot grouped PR |
| Weekly | Reliability test suite; performance suite (PT-1/3/6); gitleaks history scan |
| Monthly | Restore test (VPS script, result → backup status file); rollback drill (rehearse < 5 min) |

## 4. Branch and Release Governance

| Rule | Detail |
|------|--------|
| Branching | Trunk-based; feature branches < 3 days; no long-lived develop branch |
| Commits | Conventional Commits (`feat:`, `fix:`, `docs:`, `chore:`, ...); commitlint in CI |
| Versioning | Calendar tags `vYYYY.MM.DD-N`; no semver promise at MVP (API versioned separately as v1) |
| Release notes | Auto-generated from conventional commits (git-cliff) on tag |
| Artifact signing | cosign keyless signing of release binary + image (cheap, portfolio-grade provenance) |
| Dependency updates | Dependabot weekly (grouped minors); majors manual; lockfiles committed |
| Secret scanning | gitleaks PR gate + weekly history scan + GitHub secret scanning enabled |
| Deployment approvals | Production deploy job requires manual approval (single operator = self-approval, deliberate pause point) |
| Force push | Protected on main (never); history rewrite = incident |

## 5. Rollback Conditions (automatic guidance, human decision)

Deploy auto-halts and flags rollback when: readyz not green within 30 s; smoke test failure; crash-loop detected (2 restarts in 5 min via health check). Operator executes rollback per runbook (< 5 min).

## 6. Environment Promotion

Local → CI → production. No staging (documented decision, `docs/delivery/03-environments.md`). Dashboard: Pages PR preview deployments for visual review before merge.

## 7. Pipeline Maintenance

- Pipeline changes reviewed like code (they are code).
- Total pipeline time budget: PR < 10 min, main < 20 min — breaches trigger optimization (cache, parallelism) before acceptance.
- Secrets: Actions secrets only; never echoed; rotated with the credentials they represent.

## 8. Cross-Reference

- Deployment runbook: `docs/operations/05-deployment-and-rollback.md`
- Environments: `docs/delivery/03-environments.md`
- Testing gates: `docs/testing/02-testing-strategy.md` §5
