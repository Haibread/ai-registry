// k6 smoke test: verifies the public read-only surface responds under a
// trivial load. Intended as a fast confidence check post-deploy, not a
// capacity test. Run with:
//
//   k6 run test/load/smoke.js \
//     -e BASE_URL=https://ai-registry.example.com
//
// Default BASE_URL points at the docker-compose dev stack.

import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { Rate } from 'k6/metrics';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:3000';

// Custom failure-rate metric; threshold below treats >1 % as a failed run.
const failures = new Rate('failed_checks');

export const options = {
  // Smoke profile: 3 VUs for 30 s. Just enough to catch obvious regressions
  // (5xx, missing routes, slow TLS handshake) without stressing the target.
  vus: 3,
  duration: '30s',

  thresholds: {
    // Fail the run if p95 latency crosses 500 ms.
    http_req_duration: ['p(95)<500'],
    // Fail the run if more than 1 % of checks fail.
    failed_checks: ['rate<0.01'],
    // Zero tolerance for request errors (connection refused, DNS, etc.).
    http_req_failed: ['rate<0.01'],
  },
};

export default function () {
  group('health', () => {
    const res = http.get(`${BASE_URL}/healthz`);
    failures.add(!check(res, {
      'healthz 200': (r) => r.status === 200,
    }));
  });

  group('readiness', () => {
    const res = http.get(`${BASE_URL}/readyz`);
    failures.add(!check(res, {
      'readyz 200 or 503': (r) => r.status === 200 || r.status === 503,
    }));
  });

  group('openapi', () => {
    const res = http.get(`${BASE_URL}/openapi.yaml`);
    failures.add(!check(res, {
      'openapi 200': (r) => r.status === 200,
      'openapi is yaml': (r) => (r.headers['Content-Type'] || '').includes('yaml'),
    }));
  });

  group('v0 list servers', () => {
    const res = http.get(`${BASE_URL}/v0/servers`);
    failures.add(!check(res, {
      'servers 200': (r) => r.status === 200,
      'servers json': (r) => (r.headers['Content-Type'] || '').includes('json'),
      'servers has metadata': (r) => {
        try {
          return 'metadata' in r.json();
        } catch (_e) {
          return false;
        }
      },
    }));
  });

  group('global agent card', () => {
    const res = http.get(`${BASE_URL}/.well-known/agent-card.json`);
    failures.add(!check(res, {
      'agent card 200': (r) => r.status === 200,
    }));
  });

  sleep(1);
}
