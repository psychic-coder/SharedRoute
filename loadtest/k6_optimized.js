import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 10000 },
    { duration: '60s', target: 10000 },
    { duration: '10s', target: 0 },
  ],
};

export default function () {
  const url = 'http://localhost:8081/v1/check'; // Point to the optimized instance
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
  
  sleep(1);
}
