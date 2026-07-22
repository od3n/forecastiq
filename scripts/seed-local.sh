#!/usr/bin/env bash
# seed-local.sh — seed reference data into the local database (idempotent).
# Loads .env.local for FIQ_DATABASE_URL, then runs the binary's seed command.
set -euo pipefail
cd "$(dirname "$0")/.."

if [ -f .env.local ]; then
  set -a
  # shellcheck disable=SC1091
  source .env.local
  set +a
fi

go run ./cmd/forecastiq seed
echo "seed completed (system workspace, providers, Open-Meteo config, Johor Bahru)"
