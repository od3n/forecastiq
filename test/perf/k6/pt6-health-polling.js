// k6 Performance Test — PT-6: Health Assembly Under Polling
// 2 concurrent operators, 60s polling, 10 min. Target: p95 < 200ms.
// Reference: docs/testing/04-performance-testing.md §2
import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080/api/v1';
const ADMIN_TOKEN = __ENV.ADMIN_TOKEN || 'dev-admin-token';

export const options = {
  vus: 2,
  duration: '10m',
  thresholds: {
    http_req_duration: ['p(95)<200'],
    http_req_failed: ['rate<0.01'],
  },
};

export default function () {
  const res = http.get(`${BASE_URL}/admin/health`, {
    headers: { Authorization: `Bearer ${ADMIN_TOKEN}` },
  });

  // Parse once, defensively: JSON.parse inside a check callback throws on
  // 401/5xx or empty bodies, aborting the iteration instead of failing the
  // check cleanly (DRB-WP26 finding).
  let body = null;
  if (res.status === 200) {
    try {
      body = JSON.parse(res.body);
    } catch (_) {
      body = null;
    }
  }

  check(res, {
    'health 200': (r) => r.status === 200,
    'has cells': () => body !== null && body.data && body.data.cells !== undefined,
    'has system': () => body !== null && body.data && body.data.system !== undefined,
  });
  sleep(60); // 60s polling interval
}
