# Contribution Guide

## Workflow

- **Trunk-based** on `main` with short-lived feature branches (< 3 days). No
  long-lived `develop` branch (CI/CD doc §4).
- Open a PR; all CI jobs are blocking.
- Work proceeds by **work package** (`docs/planning/05-implementation-work-packages.md`)
  in the approved sequence; each package exits only when its quality gates pass.

## Commits

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(collection): add Open-Meteo adapter retry policy
fix(scheduler): qualify RETURNING columns in slot claim
docs(api): document the freshness block
chore(ci): bump golangci-lint to v1.64.8
```

Keep commits **slice-based**: each commit is a coherent, independently reviewable
unit (e.g. "repo skeleton + Makefile + compose", "lint config + CI", "logging +
healthz"). Avoid mixing unrelated changes.

## Before opening a PR

```bash
make fmt          # gofmt + goimports
make lint         # golangci-lint (must be zero warnings)
make test         # unit tests (race)
make test-integration   # if you touched DB/API/migrations
```

## Quality gates (binding, ADR-032)

A change is not complete without:
1. Unit coverage ≥ 80% on touched packages (formulas: 100% + property tests).
2. Contract tests for any adapter touched (old + new fixtures).
3. Integration tests for any endpoint/migration touched.
4. Zero golangci-lint warnings (includes depguard module-boundary rules).
5. OpenAPI spec valid and up to date (`make docs`).
6. Golden path green.
7. No skipped tests without a tracked issue reference.

## Coding standards

- **Idiomatic Go**; dependencies point inward (handlers → use cases → domain → ports).
- Every package has a doc comment stating its purpose and dependency rules.
- No dead code; no `TODO` without an issue reference.
- Handlers contain **no business logic** (call use cases; assemble envelopes).
- Parameterized queries only (no string SQL); secrets never logged or returned.
- Domain packages import only stdlib + the UUID kernel.

## Architecture changes

Material architecture changes are recorded as an ADR (immutable once Accepted;
supersede with a new ADR). Silent drift is not permitted — a deviation during
implementation returns as an ADR (Phase 1 report §17, condition 5).

## Documentation

`docs/` is authoritative and changes with the same review rigor as code. Keep
code-level READMEs minimal (point to `docs/`); avoid parallel documentation that
can drift.
