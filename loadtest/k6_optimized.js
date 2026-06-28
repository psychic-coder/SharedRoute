import http from 'k6/http';
import { check, sleep } from 'k6';

// OPTIMIZED: Local-cache-warmed path.
// Targets shardroute-1 (port 8081). After the first request per VU,
// subsequent requests are served from the in-process ApproximateCounter
// (no Redis round-trip), demonstrating the local cache latency benefit.
export const options = {
  stages: [
    { duration: '30s', target: 500 },
    { duration: '60s', target: 500 },
    { duration: '10s', target: 0 },
  ],
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(50)', 'p(95)', 'p(99)', 'count'],
};

export default function () {
  const url = 'http://localhost:8081/v1/check';
  const payload = JSON.stringify({
    key: 'user_' + __VU,
    cost: 1,
    limit_name: 'api_requests'
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };

  const res = http.post(url, payload, params);
  
  check(res, {
    'is status 200 or 429': (r) => r.status === 200 || r.status === 429,
  });
  
  sleep(0.1);
}
