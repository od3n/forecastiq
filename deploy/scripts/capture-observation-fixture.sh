#!/usr/bin/env bash
# capture-observation-fixture.sh — record a real Open-Meteo Historical response
# as an observation contract fixture (contract testing doc §1.1/§1.3). Run
# manually; sanitize before committing (Open-Meteo is keyless, no account ids).
#
# Usage: deploy/scripts/capture-observation-fixture.sh [name] [start_hour] [end_hour]
#   name:       fixture basename (default: historical_success_v1)
#   start_hour: ISO UTC hour (default: 48h ago, top of hour)
#   end_hour:   ISO UTC hour (default: now, top of hour)
set -euo pipefail
cd "$(dirname "$0")/../.."

NAME="${1:-historical_success_v1}"
START="${2:-$(date -u -v-48H +%Y-%m-%dT%H:00 2>/dev/null || date -u -d '48 hours ago' +%Y-%m-%dT%H:00)}"
END="${3:-$(date -u +%Y-%m-%dT%H:00)}"
OUT="test/fixtures/openmeteo-historical/${NAME}.json"

# Johor Bahru coordinates; measured hourly variables matching the adapter; UTC.
URL="https://historical-forecast-api.open-meteo.com/v1/forecast?latitude=1.4927&longitude=103.7414"
URL="${URL}&hourly=temperature_2m,relative_humidity_2m,wind_speed_10m,wind_direction_10m,surface_pressure,precipitation,weather_code"
URL="${URL}&start_hour=${START}&end_hour=${END}&timezone=UTC"

echo "capturing Open-Meteo Historical response → ${OUT}"
curl -sf "${URL}" -o "${OUT}"
echo "done. Review + sanitize, then commit as a contract fixture."
