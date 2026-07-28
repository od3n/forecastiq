#!/usr/bin/env python3
"""Fault-injectable fake Open-Meteo provider (WP-26b reliability suite).

Serves a minimal, VALID openmeteo-v1 forecast response with FRESH hourly
timestamps (recorded fixtures carry past times, which the domain rejects via
target_time > issued_at). Modes, selected by the MODE env var:

  ok    — respond 200 with a 24-hour valid payload (default)
  hang  — accept the connection and never respond (provider-timeout injection)

Stdlib only; run inside any python:3-alpine container on the compose network:

  docker run -d --name fiq-fakeprovider --network <net> \
    -v $PWD/test/perf/fakeprovider.py:/srv/fakeprovider.py:ro \
    -e MODE=hang python:3.12-alpine python /srv/fakeprovider.py
"""

import datetime
import json
import os
import time
from http.server import BaseHTTPRequestHandler, HTTPServer

MODE = os.environ.get("MODE", "ok")
PORT = int(os.environ.get("PORT", "8080"))
HOURS = 24


def payload() -> bytes:
    start = datetime.datetime.now(datetime.timezone.utc).replace(
        minute=0, second=0, microsecond=0
    ) + datetime.timedelta(hours=1)
    times, temps, feels, prob, precip, hum, wind, wdir, press, cloud, code = (
        [], [], [], [], [], [], [], [], [], [], []
    )
    for i in range(HOURS):
        t = start + datetime.timedelta(hours=i)
        times.append(t.strftime("%Y-%m-%dT%H:%M"))
        temps.append(round(27.0 + (i % 8) * 0.7, 1))
        feels.append(round(29.0 + (i % 8) * 0.7, 1))
        prob.append((i * 7) % 100)
        precip.append(round((i % 5) * 0.4, 1))
        hum.append(70 + (i % 20))
        wind.append(round(2.0 + (i % 4) * 0.5, 1))
        wdir.append((i * 30) % 360)
        press.append(round(1008.0 + (i % 6) * 0.5, 1))
        cloud.append((i * 11) % 100)
        code.append(3 if precip[-1] == 0 else 61)
    return json.dumps({
        "latitude": 1.4927,
        "longitude": 103.7414,
        "utc_offset_seconds": 0,
        "timezone": "UTC",
        "hourly": {
            "time": times,
            "temperature_2m": temps,
            "apparent_temperature": feels,
            "precipitation_probability": prob,
            "precipitation": precip,
            "relative_humidity_2m": hum,
            "wind_speed_10m": wind,
            "wind_direction_10m": wdir,
            "surface_pressure": press,
            "cloud_cover": cloud,
            "weather_code": code,
        },
    }).encode()


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):  # noqa: N802 (stdlib naming)
        if MODE == "hang":
            time.sleep(3600)  # hold the socket open; client times out
            return
        body = payload()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, fmt, *args):  # quiet
        pass


if __name__ == "__main__":
    print(f"fakeprovider: MODE={MODE} port={PORT}", flush=True)
    HTTPServer(("0.0.0.0", PORT), Handler).serve_forever()
