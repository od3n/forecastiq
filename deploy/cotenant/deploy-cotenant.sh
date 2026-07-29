#!/usr/bin/env bash
# deploy/cotenant/deploy-cotenant.sh — deploy ForecastIQ as a co-tenant on the
# shared host (runs FROM the operator machine; SSHes to the host).
#
# Prereqs on the host (once): /opt/forecastiq/shared/{db_password,secrets.env},
# the forecastiq DB (deploy/cotenant/setup-shared-db.sh), and Docker.
#
# Env:
#   HOST         ssh target (default ubuntu@56.68.14.152)
#   SSH_KEY      path to key (default ~/.ssh/od3n.com-key.pem)
#   IMAGE        image ref to deploy (default ghcr.io/od3n/forecastiq:latest)
#   GHCR_TOKEN   read:packages token for the host pull (required for private pkg)
set -euo pipefail

HOST="${HOST:-ubuntu@56.68.14.152}"
SSH_KEY="${SSH_KEY:-$HOME/.ssh/od3n.com-key.pem}"
IMAGE="${IMAGE:-ghcr.io/od3n/forecastiq:latest}"
GHCR_TOKEN="${GHCR_TOKEN:-}"
SSH="ssh -o StrictHostKeyChecking=accept-new -i ${SSH_KEY} ${HOST}"
HERE="$(cd "$(dirname "$0")" && pwd)"

echo "[1/6] Uploading co-tenant compose + proxy config..."
scp -q -i "$SSH_KEY" "$HERE/docker-compose.cotenant.yml" "${HOST}:/opt/forecastiq/docker-compose.yml"
scp -q -i "$SSH_KEY" "$HERE/nginx.conf" "${HOST}:/opt/forecastiq/nginx.conf"

echo "[2/6] Composing /opt/forecastiq/.env (image + DB URL from host password)..."
$SSH bash <<REMOTE
  set -euo pipefail
  PW=\$(cat /opt/forecastiq/shared/db_password)
  {
    echo "FIQ_IMAGE=${IMAGE}"
    echo "FIQ_DATABASE_URL=postgres://forecastiq:\${PW}@host.docker.internal:5432/forecastiq?sslmode=disable"
    echo "FIQ_DASHBOARD_ORIGIN=https://forecastiq.od3n.com"
  } > /opt/forecastiq/.env
  chmod 600 /opt/forecastiq/.env
  touch /opt/forecastiq/shared/backup-status.json
  mkdir -p /opt/forecastiq/payloads
  # The app runs as distroless nonroot (uid 65532) and must write payloads;
  # the host dir defaults to the deploy user, so hand it to the container uid
  # (mirrors deploy/bootstrap.sh for the standalone topology).
  sudo chown -R 65532:65532 /opt/forecastiq/payloads
REMOTE

echo "[3/6] Refreshing Cloudflare real-IP ranges in nginx.conf..."
$SSH bash <<'REMOTE'
  set -euo pipefail
  RANGES=$(mktemp)
  { curl -fsS https://www.cloudflare.com/ips-v4; echo; curl -fsS https://www.cloudflare.com/ips-v6; } \
    | sed '/^$/d;s/^/set_real_ip_from /;s/$/;/' > "$RANGES"
  # Insert the ranges right after the marker line, replacing any prior block.
  awk -v r="$(sed 's/[&/\]/\\&/g' "$RANGES" | paste -sd'\n' -)" '
    /# CF_REAL_IP_RANGES/ { print; print r; skip=1; next }
    /^$/ && skip { skip=0 }
    skip && /set_real_ip_from/ { next }
    { print }
  ' /opt/forecastiq/nginx.conf > /opt/forecastiq/nginx.conf.new
  mv /opt/forecastiq/nginx.conf.new /opt/forecastiq/nginx.conf
  rm -f "$RANGES"
REMOTE

echo "[4/6] Pulling image + running migrations..."
$SSH bash <<REMOTE
  set -euo pipefail
  cd /opt/forecastiq
  if [ -n "${GHCR_TOKEN}" ]; then
    echo "${GHCR_TOKEN}" | docker login ghcr.io -u token --password-stdin >/dev/null
  fi
  docker compose pull api
  docker compose run --rm api migrate up
REMOTE

echo "[5/6] Starting api + proxy..."
$SSH "cd /opt/forecastiq && docker compose up -d"

echo "[6/6] Waiting for readiness (via the proxy on :8080)..."
$SSH bash <<'REMOTE'
  set -euo pipefail
  for i in $(seq 1 30); do
    if curl -sf -H 'Host: forecastiq-api.od3n.com' http://127.0.0.1:8080/readyz >/dev/null 2>&1; then
      echo "  readyz OK after ${i}s"; exit 0
    fi
    sleep 1
  done
  echo "  ERROR: /readyz not green after 30s"; docker compose -f /opt/forecastiq/docker-compose.yml logs --tail 40 api; exit 1
REMOTE

echo ""
echo "=== Co-tenant deploy complete: ${IMAGE} ==="
