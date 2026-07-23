#!/usr/bin/env bash
# capture-openweather-fixture.sh — record a real OpenWeather One Call 3.0
# response as a test fixture (contract testing doc §1.1/§1.3). Run manually;
# sanitize before committing (strip any account identifiers; the appid must
# NEVER appear in the committed fixture — it is only used to make the call).
#
# Usage: FIQ_PROVIDER_OPENWEATHER_API_KEY=... \
#          deploy/scripts/capture-openweather-fixture.sh [name]
#   name: fixture basename (default: onecall_success_v3)
set -euo pipefail
cd "$(dirname "$0")/../.."

: "${FIQ_PROVIDER_OPENWEATHER_API_KEY:?set FIQ_PROVIDER_OPENWEATHER_API_KEY}"
NAME="${1:-onecall_success_v3}"
OUT="test/fixtures/openweather/${NAME}.json"

# Johor Bahru coordinates; hourly-only; metric units; UTC epoch timestamps.
URL="https://api.openweathermap.org/data/3.0/onecall?lat=1.4927&lon=103.7414"
URL="${URL}&exclude=current,minutely,daily,alerts&units=metric"
URL="${URL}&appid=${FIQ_PROVIDER_OPENWEATHER_API_KEY}"

echo "capturing OpenWeather One Call response → ${OUT}"
curl -sf "${URL}" -o "${OUT}"
echo "done. Review + sanitize (remove any account identifiers), then commit as a contract fixture."
