import http from 'k6/http';
import { check, sleep } from 'k6';

// BASELINE: shardroute-direct (cache_mode=direct).
// Every request executes a Redis Lua EVALSHA round-trip — no in-process state.
// Override port: k6 run k6_baseline.js --env TARGET_PORT=8081
const PORT = __ENV.TARGET_PORT || '8081';

export const options = {
  stages: [
    { duration: '30s', target: 200 },
    { duration: '60s', target: 200 },
    { duration: '10s', target: 0 },
  ],
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(50)', 'p(95)', 'p(99)', 'count'],
};

export default function () {
  const url = `http://localhost:${PORT}/v1/check`;
  const payload = JSON.stringify({
    key: 'user_' + __VU,
    cost: 1,
    limit_name: 'api_requests',
  });

  const params = {
    headers: { 'Content-Type': 'application/json' },
  };

  const res = http.post(url, payload, params);

  check(res, {
    'is status 200 or 429': (r) => r.status === 200 || r.status === 429,
  });

  sleep(0.1);
}
