#!/usr/bin/env bash
# capture-fixture.sh — record a real Open-Meteo forecast response as a test
# fixture (contract testing doc §1.1/§1.3). Run manually; sanitize before
# committing (no account identifiers — Open-Meteo is keyless).
#
# Usage: deploy/scripts/capture-fixture.sh [name]
#   name: fixture basename (default: forecast_success_v1)
set -euo pipefail
cd "$(dirname "$0")/../.."

NAME="${1:-forecast_success_v1}"
OUT="test/fixtures/openmeteo/${NAME}.json"

# Johor Bahru coordinates; hourly variables matching the adapter; UTC; 7 days.
URL="https://api.open-meteo.com/v1/forecast?latitude=1.4927&longitude=103.7414"
URL="${URL}&hourly=temperature_2m,apparent_temperature,precipitation_probability,precipitation,relative_humidity_2m,wind_speed_10m,wind_direction_10m,surface_pressure,cloud_cover,weather_code"
URL="${URL}&forecast_days=7&timezone=UTC"

echo "capturing Open-Meteo response → ${OUT}"
curl -sf "${URL}" -o "${OUT}"
echo "done. Review + sanitize, then commit as a contract fixture."
