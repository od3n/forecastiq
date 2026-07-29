#!/usr/bin/env bash
# deploy/cotenant/setup-shared-db.sh — create the forecastiq role + database
# inside the co-tenant host's existing (codex-usage) PostgreSQL container.
#
# Co-tenant deployment (ForecastIQ shares a host with another app): rather than
# run a second Postgres on a 2 GB box, ForecastIQ gets an isolated `forecastiq`
# database + role in the neighbouring app's Postgres. Fully idempotent.
#
# The password is read from a host-only file (never passed on the CLI or
# committed). Run on the host as the deploy user.
#
# Env overrides:
#   NEIGHBOR_COMPOSE  compose invocation for the neighbour stack that owns PG
#   PW_FILE           path to the forecastiq DB password file (mode 0600)
set -euo pipefail

NEIGHBOR_COMPOSE="${NEIGHBOR_COMPOSE:-docker compose -p app -f /opt/codex-usage/current/docker-compose.yml}"
PW_FILE="${PW_FILE:-/opt/forecastiq/shared/db_password}"
PG_SUPERUSER="${PG_SUPERUSER:-codex_usage}"

if [ ! -f "$PW_FILE" ]; then
  echo "ERROR: password file $PW_FILE not found (generate: openssl rand -hex 24 > $PW_FILE; chmod 600)" >&2
  exit 1
fi
PW=$(cat "$PW_FILE")

psql_su() { $NEIGHBOR_COMPOSE exec -T postgres psql -U "$PG_SUPERUSER" -d postgres -tAc "$1"; }

if [ "$(psql_su "SELECT 1 FROM pg_roles WHERE rolname='forecastiq'")" = "1" ]; then
  psql_su "ALTER ROLE forecastiq LOGIN PASSWORD '$PW'" >/dev/null
  echo "ROLE_ALTERED"
else
  psql_su "CREATE ROLE forecastiq LOGIN PASSWORD '$PW'" >/dev/null
  echo "ROLE_CREATED"
fi

if [ "$(psql_su "SELECT 1 FROM pg_database WHERE datname='forecastiq'")" = "1" ]; then
  echo "DB_EXISTS"
else
  psql_su "CREATE DATABASE forecastiq OWNER forecastiq" >/dev/null
  echo "DB_CREATED"
fi

$NEIGHBOR_COMPOSE exec -T postgres \
  psql "postgres://forecastiq:${PW}@localhost:5432/forecastiq" \
  -tAc "SELECT 'login_ok='||current_database()"
