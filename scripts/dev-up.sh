#!/usr/bin/env bash
# dev-up.sh — one-command local startup.
# Ensures a local env file exists, then starts PostgreSQL + the app (hot reload)
# via docker compose. The app auto-migrates and auto-seeds on boot.
set -euo pipefail
cd "$(dirname "$0")/.."

if [ ! -f .env.local ]; then
  echo "creating .env.local from .env.example"
  cp .env.example .env.local
fi

docker compose up --build -d
echo
echo "ForecastIQ stack starting:"
echo "  API:     http://localhost:8080   (health: /healthz, readiness: /readyz)"
echo "  Metrics: http://localhost:9090/metrics"
echo "  Logs:    make dev-logs"
