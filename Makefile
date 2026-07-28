# ─────────────────────────────────────────────────────────────────────
# ForecastIQ — developer workflow
# Run `make help` for the target index. One-command local startup: `make dev-up`.
# ─────────────────────────────────────────────────────────────────────

SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

# ── Paths & versions ──────────────────────────────────────────────────
ROOT_DIR      := $(CURDIR)
BIN_DIR       := $(ROOT_DIR)/bin
TOOLS_DIR     := $(ROOT_DIR)/tools/bin
APP_PKG       := ./cmd/forecastiq
APP_BIN       := $(BIN_DIR)/forecastiq

GOLANGCI_LINT_VERSION ?= v1.64.8
SWAG_VERSION          ?= v1.16.3
GOIMPORTS_VERSION     ?= v0.24.0

GO        ?= go
DOCKER    ?= docker
COMPOSE   ?= $(DOCKER) compose

# Build metadata (calendar versioning per CI/CD doc §4)
VERSION   ?= $(shell date -u +%Y.%m.%d)-dev
COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILDDATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS   := -s -w \
	-X github.com/forecastiq/forecastiq/internal/platform/buildinfo.Version=$(VERSION) \
	-X github.com/forecastiq/forecastiq/internal/platform/buildinfo.Commit=$(COMMIT) \
	-X github.com/forecastiq/forecastiq/internal/platform/buildinfo.BuildDate=$(BUILDDATE)

# Test selection
INTEGRATION_TAG := integration

# ── Help ──────────────────────────────────────────────────────────────
.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ── Setup & tooling ───────────────────────────────────────────────────
.PHONY: setup
setup: tools ## Install Go tooling and prepare local env file
	@if [ ! -f .env.local ]; then cp .env.example .env.local && echo "created .env.local from .env.example"; fi
	@echo "Setup complete. Run 'make dev-up' to start the stack."

.PHONY: tools
tools: ## Install pinned dev tools into tools/bin
	@mkdir -p $(TOOLS_DIR)
	GOBIN=$(TOOLS_DIR) $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	GOBIN=$(TOOLS_DIR) $(GO) install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)

# ── Build & run ───────────────────────────────────────────────────────
.PHONY: build
build: ## Compile the single binary into bin/
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(APP_BIN) $(APP_PKG)

.PHONY: run
run: ## Run the binary locally (mode from FIQ_MODE / --mode)
	$(GO) run $(APP_PKG) serve --mode=all

.PHONY: tidy
tidy: ## go mod tidy
	$(GO) mod tidy

# ── Local stack (docker compose) ──────────────────────────────────────
.PHONY: dev-up
dev-up: ## Start postgres + app (auto-migrate + seed) via docker compose
	$(COMPOSE) up --build -d
	@echo "Stack starting. App: http://localhost:8080  Metrics: http://localhost:9090/metrics"

.PHONY: dev-down
dev-down: ## Stop the local stack (keeps volumes)
	$(COMPOSE) down

.PHONY: dev-reset
dev-reset: ## Destroy volumes and restart clean (re-migrate + reseed)
	$(COMPOSE) down -v --remove-orphans
	$(COMPOSE) up --build -d

.PHONY: dev-logs
dev-logs: ## Tail application logs
	$(COMPOSE) logs -f app

.PHONY: start
start: dev-up ## Alias: start the stack

# ── Observability stack ───────────────────────────────────────────────
.PHONY: obs-up
obs-up: ## Start the observability stack (Grafana + Prometheus + Loki)
	$(COMPOSE) --profile obs up -d prometheus loki promtail grafana
	@echo "Grafana: http://localhost:3000  Prometheus: http://localhost:9091"

.PHONY: obs-down
obs-down: ## Stop the observability stack (obs services only)
	$(COMPOSE) --profile obs rm -sf prometheus loki promtail grafana

.PHONY: obs-reset
obs-reset: ## Destroy observability volumes and restart clean
	$(COMPOSE) --profile obs rm -sf prometheus loki promtail grafana
	-docker volume rm -f forecastiq_prometheus_data forecastiq_loki_data forecastiq_grafana_data
	$(COMPOSE) --profile obs up -d prometheus loki promtail grafana

.PHONY: stop
stop: dev-down ## Alias: stop the stack

# ── Performance / reliability (WP-26b; isolated fiqperf project) ─────
PERF_COMPOSE := $(COMPOSE) -p fiqperf -f docker-compose.yml -f test/perf/compose.perf.yml
PERF_DB      := postgres://forecastiq:forecastiq@localhost:25432/forecastiq?sslmode=disable
PERF_URL     := http://localhost:28080
PERF_PRESET  := base

.PHONY: perf-up
perf-up: ## Start the isolated perf stack (ports 28080/29090/25432)
	$(PERF_COMPOSE) up -d --build postgres app
	@echo "Perf stack: $(PERF_URL)  (PERF_RATE_LIMIT=100000 make perf-up for the k6 env)"

.PHONY: perf-down
perf-down: ## Destroy the perf stack including volumes
	$(PERF_COMPOSE) down -v --remove-orphans

.PHONY: perf-seed
perf-seed: ## Seed the perf dataset (PERF_PRESET=base|extended|analysis)
	$(GO) run ./test/perf/seeder --preset=$(PERF_PRESET) --reset --db "$(PERF_DB)"

.PHONY: perf-pt3
perf-pt3: ## PT-3 ingestion burst (NFR-P07)
	$(GO) run ./test/perf/pt3 --db "$(PERF_DB)"

.PHONY: perf-pt4
perf-pt4: ## PT-4 analysis batch < 10 min (seed analysis preset first)
	bash test/perf/pt4-analysis-batch.sh $(PERF_URL)

.PHONY: perf-pt7
perf-pt7: ## PT-7 query baselines p95 < 100 ms (NFR-P08)
	$(GO) run ./test/perf/pt7 --db "$(PERF_DB)"

.PHONY: perf-k6
perf-k6: ## Run one k6 scenario (K6_SCRIPT=pt1-dashboard-mix.js; needs PERF_RATE_LIMIT env)
	docker run --rm --network fiqperf_default \
		-e BASE_URL=http://app:8080/api/v1 \
		-e LOCATION_ID=00000000-0000-0000-0001-000000000000 \
		-e ADMIN_TOKEN=perf-admin-token \
		-v "$(PWD)/test/perf/k6:/scripts:ro" grafana/k6:0.57.0 run /scripts/$(K6_SCRIPT)

.PHONY: perf-reliability
perf-reliability: ## Reliability suites (request-path + fault injection; DEFAULT limiter)
	ADMIN_TOKEN=perf-admin-token bash test/perf/reliability.sh $(PERF_URL)
	COMPOSE="$(PERF_COMPOSE)" NETWORK=fiqperf_default bash test/perf/reliability-faults.sh $(PERF_URL)

# ── Database ──────────────────────────────────────────────────────────
.PHONY: migrate
migrate: ## Apply all pending migrations
	$(GO) run $(APP_PKG) migrate up

.PHONY: migrate-down
migrate-down: ## Roll back the most recent migration
	$(GO) run $(APP_PKG) migrate down 1

.PHONY: migrate-force
migrate-force: ## Clear a dirty migration state (use with care)
	$(GO) run $(APP_PKG) migrate force

.PHONY: seed
seed: ## Seed system workspace, providers, configuration, JB location
	$(GO) run $(APP_PKG) seed

# ── Quality gates ─────────────────────────────────────────────────────
.PHONY: fmt
fmt: ## Format code (gofmt + goimports)
	@$(TOOLS_DIR)/goimports -w -local github.com/forecastiq/forecastiq $$(find . -name '*.go' -not -path './tools/*')

.PHONY: fmt-check
fmt-check: ## Fail if code is unformatted
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './tools/*'))" || \
		(echo "gofmt needed:" && gofmt -l $$(find . -name '*.go' -not -path './tools/*') && exit 1)

.PHONY: vet
vet: ## go vet
	$(GO) vet ./...

.PHONY: lint
lint: ## golangci-lint (incl. depguard module-boundary rules)
	@$(TOOLS_DIR)/golangci-lint run ./...

.PHONY: test
test: ## Unit tests (race detector on)
	$(GO) test -race -count=1 ./...

.PHONY: test-integration
test-integration: ## Integration tests (requires Docker; testcontainers)
	$(GO) test -tags $(INTEGRATION_TAG) -count=1 -timeout 20m ./...

.PHONY: test-all
test-all: test test-integration ## Unit + integration

.PHONY: test-coverage
test-coverage: ## Unit tests with coverage report
	@mkdir -p coverage
	$(GO) test -race -coverprofile=coverage/coverage.out -covermode=atomic ./...
	$(GO) tool cover -func=coverage/coverage.out | tail -1

# ── API docs / OpenAPI ────────────────────────────────────────────────
# The OpenAPI 3.1 spec is maintained at api/openapi/openapi.json and served at
# /api/v1/openapi.json. `make docs` validates it is well-formed and complete.
.PHONY: docs
docs: ## Validate the committed OpenAPI spec
	@python3 -c "import json; s=json.load(open('api/openapi/openapi.json')); assert s['openapi'].startswith('3.1'), 'must be OpenAPI 3.1'; req=['/locations','/providers','/forecasts/latest','/admin/collections/trigger','/forecast-collections','/rankings','/rankings/methodology','/accuracy/summary','/accuracy','/forecast-comparison','/admin/health','/me','/api-keys','/api-keys/{id}','/admin/users','/auth/webhook','/me/export','/exports/{id}']; missing=[p for p in req if p not in s['paths']]; assert not missing, 'missing paths: '+str(missing); print('OpenAPI valid:', len(s['paths']), 'paths')"

.PHONY: api-gen
api-gen: docs ## Alias: validate API artifacts

.PHONY: docs-check
docs-check: ## CI gate: committed OpenAPI spec must be valid
	@$(MAKE) --no-print-directory docs

# ── Container ─────────────────────────────────────────────────────────
.PHONY: docker-build
docker-build: ## Build the production (distroless) image
	$(DOCKER) build -t forecastiq:$(VERSION) .

# ── Deploy ────────────────────────────────────────────────────────────
.PHONY: deploy-release
deploy-release: ## Build + push a release image to GHCR (ADR-033: image-based deploy)
	$(DOCKER) build --target prod -t ghcr.io/od3n/forecastiq:$(VERSION) .
	$(DOCKER) push ghcr.io/od3n/forecastiq:$(VERSION)
	@echo "Release image: ghcr.io/od3n/forecastiq:$(VERSION)"
	@echo "Deploy: bash deploy/scripts/deploy.sh ghcr.io/od3n/forecastiq:$(VERSION)"

.PHONY: deploy-smoke
deploy-smoke: ## Run smoke tests against a running instance (default: localhost:8080)
	@bash deploy/scripts/smoke-test.sh $(SMOKE_URL)

# ── Housekeeping ──────────────────────────────────────────────────────
.PHONY: clean
clean: ## Remove build/test artifacts
	rm -rf $(BIN_DIR) coverage /tmp/fiq-openapi checksums.txt
