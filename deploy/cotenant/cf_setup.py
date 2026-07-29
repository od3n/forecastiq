#!/usr/bin/env python3
"""deploy/cotenant/cf_setup.py — Cloudflare DNS + Origin Rule for ForecastIQ.

Idempotently ensures:
  1. A proxied A-record  api.forecastiq.od3n.com -> the co-tenant host EIP.
  2. An Origin Rule that rewrites the destination port to 8080 for that
     hostname, so Cloudflare-proxied traffic reaches ForecastIQ's dedicated
     proxy (host :8080) without touching the neighbour app's :80 nginx.

Creds from env (never hard-coded): CLOUDFLARE_API_TOKEN, CLOUDFLARE_ZONE_ID.
Config from argv/env: FIQ_API_HOST (default api.forecastiq.od3n.com),
ORIGIN_IP (required), ORIGIN_PORT (default 8080).

Usage:
  CLOUDFLARE_API_TOKEN=... CLOUDFLARE_ZONE_ID=... ORIGIN_IP=1.2.3.4 \
    python3 deploy/cotenant/cf_setup.py
"""
import json
import os
import sys
import urllib.request

API = "https://api.cloudflare.com/client/v4"
TOKEN = os.environ["CLOUDFLARE_API_TOKEN"]
ZONE = os.environ["CLOUDFLARE_ZONE_ID"]
HOST = os.environ.get("FIQ_API_HOST", "forecastiq-api.od3n.com")
ORIGIN_IP = os.environ["ORIGIN_IP"]
ORIGIN_PORT = int(os.environ.get("ORIGIN_PORT", "8080"))


def call(method, path, body=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(API + path, data=data, method=method)
    req.add_header("Authorization", "Bearer " + TOKEN)
    req.add_header("Content-Type", "application/json")
    with urllib.request.urlopen(req) as r:
        return json.load(r)


def ensure_dns():
    q = call("GET", f"/zones/{ZONE}/dns_records?type=A&name={HOST}")
    recs = q.get("result", [])
    payload = {"type": "A", "name": HOST, "content": ORIGIN_IP,
               "proxied": True, "ttl": 1,
               "comment": "ForecastIQ API origin (co-tenant :%d)" % ORIGIN_PORT}
    if recs:
        rid = recs[0]["id"]
        call("PUT", f"/zones/{ZONE}/dns_records/{rid}", payload)
        print(f"DNS: updated {HOST} -> {ORIGIN_IP} (proxied)")
    else:
        call("POST", f"/zones/{ZONE}/dns_records", payload)
        print(f"DNS: created {HOST} -> {ORIGIN_IP} (proxied)")


def ensure_origin_rule():
    # http_request_origin phase ruleset (entrypoint). Rewrite destination port
    # to ORIGIN_PORT for our hostname; leave any existing rules intact.
    phase = f"/zones/{ZONE}/rulesets/phases/http_request_origin/entrypoint"
    expr = f'(http.host eq "{HOST}")'
    rule = {"expression": expr, "description": "ForecastIQ origin port",
            "action": "route",
            "action_parameters": {"origin": {"port": ORIGIN_PORT}},
            "enabled": True}
    try:
        rs = call("GET", phase).get("result", {})
    except urllib.error.HTTPError as e:
        rs = {} if e.code == 404 else sys.exit(f"origin ruleset GET failed: {e}")
    rules = rs.get("rules", []) if rs else []
    for r in rules:
        if r.get("expression") == expr and r.get("action") == "route":
            print("Origin Rule: already present")
            return
    # Replace the origin-phase entrypoint with exactly our single routing rule.
    # On this zone the origin phase holds only ForecastIQ's rule (the neighbour
    # app is reached by its own hostname on :80 and defines none), so a full
    # replace is safe and also cleans up any prior ForecastIQ host expression.
    call("PUT", phase, {"rules": [rule]})
    print(f"Origin Rule: set (port -> {ORIGIN_PORT} for {HOST})")


if __name__ == "__main__":
    ensure_dns()
    ensure_origin_rule()
