// k6 Performance Test — PT-1: Dashboard Read Mix
// 100 VU, 10 min. Target: p50 < 50ms, p95 < 200ms, p99 < 500ms, 0 errors.
// Reference: docs/testing/04-performance-testing.md §2
//
// ENVIRONMENT REQUIREMENT: all k6 traffic originates from ONE source IP, so
// the per-IP rate limiter (FIQ_RATE_LIMIT_PER_IP_PER_MIN, default 120/min)
// will 429 this load long before 100 VU. The perf environment MUST raise it,
// e.g. FIQ_RATE_LIMIT_PER_IP_PER_MIN=100000, or thresholds fail for reasons
// unrelated to performance (DRB-WP26 finding).
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080/api/v1';
// Default: first seeded perf location (test/perf/seeder; perfids.LocationID(0)).
const LOCATION_ID = __ENV.LOCATION_ID || '00000000-0000-0000-0001-000000000000';

export const options = {
  stages: [
    { duration: '1m', target: 100 },  // ramp up
    { duration: '8m', target: 100 },  // sustained
    { duration: '1m', target: 0 },    // ramp down
  ],
  thresholds: {
    http_req_duration: ['p(50)<50', 'p(95)<200', 'p(99)<500'],
    http_req_failed: ['rate==0'],   // doc §2 PT-1 target: 0 errors
  },
};

const errorRate = new Rate('errors');
const rankingDuration = new Trend('ranking_duration');
const summaryDuration = new Trend('summary_duration');
const trendsDuration = new Trend('trends_duration');
const fvaDuration = new Trend('fva_duration');

export default function () {
  const rand = Math.random();

  if (rand < 0.50) {
    // 50% — Rankings
    const res = http.get(`${BASE_URL}/rankings?location_id=${LOCATION_ID}&horizon_minutes=60`);
    check(res, { 'rankings 200': (r) => r.status === 200 });
    errorRate.add(res.status !== 200);
    rankingDuration.add(res.timings.duration);
  } else if (rand < 0.75) {
    // 25% — Accuracy summary
    const res = http.get(`${BASE_URL}/accuracy/summary?location_id=${LOCATION_ID}`);
    check(res, { 'summary 200': (r) => r.status === 200 });
    errorRate.add(res.status !== 200);
    summaryDuration.add(res.timings.duration);
  } else if (rand < 0.90) {
    // 15% — Trends
    const res = http.get(`${BASE_URL}/accuracy?location_id=${LOCATION_ID}&granularity=daily&range=30d`);
    check(res, { 'trends 200': (r) => r.status === 200 });
    errorRate.add(res.status !== 200);
    trendsDuration.add(res.timings.duration);
  } else {
    // 10% — Forecast vs Actual
    const today = new Date().toISOString().split('T')[0];
    const res = http.get(`${BASE_URL}/forecast-comparison?location_id=${LOCATION_ID}&date=${today}&variable=temperature_2m&horizon_minutes=60`);
    check(res, { 'fva 2xx': (r) => r.status >= 200 && r.status < 300 });
    errorRate.add(res.status >= 400);
    fvaDuration.add(res.timings.duration);
  }

  sleep(0.5 + Math.random() * 0.5); // 0.5-1s think time
}
