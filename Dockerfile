# syntax=docker/dockerfile:1
# ─────────────────────────────────────────────────────────────────────
# ForecastIQ container image
#   - `dev`  target: golang + air hot-reload (used by docker-compose)
#   - `prod` target: distroless static, nonroot, CGO off (production)
# Build metadata is injected via ldflags (calendar versioning, CI/CD §4).
# ─────────────────────────────────────────────────────────────────────

# ── Base: toolchain + module cache ────────────────────────────────────
FROM golang:1.25-alpine AS base
WORKDIR /src
RUN apk add --no-cache git ca-certificates tzdata
COPY go.mod go.sum ./
RUN go mod download

# ── Dev: hot reload ───────────────────────────────────────────────────
FROM base AS dev
RUN go install github.com/air-verse/air@v1.52.3
ENV FIQ_LOG_FORMAT=text
EXPOSE 8080 9090
CMD ["air"]

# ── Build: production binary ──────────────────────────────────────────
FROM base AS build
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILDDATE=unknown
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags "-s -w \
      -X github.com/forecastiq/forecastiq/internal/platform/buildinfo.Version=${VERSION} \
      -X github.com/forecastiq/forecastiq/internal/platform/buildinfo.Commit=${COMMIT} \
      -X github.com/forecastiq/forecastiq/internal/platform/buildinfo.BuildDate=${BUILDDATE}" \
    -o /out/forecastiq ./cmd/forecastiq

# ── Prod: distroless, nonroot ─────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot AS prod
COPY --from=build /out/forecastiq /usr/local/bin/forecastiq
EXPOSE 8080 9090
USER nonroot
ENTRYPOINT ["/usr/local/bin/forecastiq"]
CMD ["serve", "--mode=all"]
