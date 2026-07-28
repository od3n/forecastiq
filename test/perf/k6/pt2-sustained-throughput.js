// k6 Performance Test — PT-2: Sustained Throughput
// Ramp to 100 req/s, 5 min hold. Target: ≥ 100 req/s at p95 < 200ms.
// Reference: docs/testing/04-performance-testing.md §2
//
// ENVIRONMENT REQUIREMENT: raise FIQ_RATE_LIMIT_PER_IP_PER_MIN in the perf
// environment (single-source-IP load vs the per-IP limiter; see PT-1 header).
import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080/api/v1';
// Default: first seeded perf location (test/perf/seeder; perfids.LocationID(0)).
const LOCATION_ID = __ENV.LOCATION_ID || '00000000-0000-0000-0001-000000000000';

export const options = {
  scenarios: {
    sustained: {
      executor: 'constant-arrival-rate',
      // 105/s target: a constant-arrival-rate of exactly 100/s aggregates
      // fractionally UNDER 100/s over the run (startup settling), so the
      // NFR-P05 'iterations rate>=100' gate could never pass at rate=100 —
      // even with every request served instantly (WP-26b baseline finding).
      // Driving 105/s proves ≥ 100 req/s sustained without relaxing the gate.
      rate: 105,
      timeUnit: '1s',
      duration: '5m',
      preAllocatedVUs: 150,
      maxVUs: 300,
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<200'],
    http_req_failed: ['rate<0.01'],
    iterations: ['rate>=100'],
  },
};

const endpoints = [
  `/rankings?location_id=${LOCATION_ID}&horizon_minutes=60`,
  `/accuracy/summary?location_id=${LOCATION_ID}`,
  `/providers`,
  `/locations`,
  `/rankings/methodology`,
];

export default function () {
  const endpoint = endpoints[Math.floor(Math.random() * endpoints.length)];
  const res = http.get(`${BASE_URL}${endpoint}`);
  check(res, { 'status 2xx': (r) => r.status >= 200 && r.status < 300 });
}
