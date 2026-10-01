import http from 'k6/http';
import { Counter } from 'k6/metrics';
const eof = new Counter('transport_errors');
const urls = ['http://localhost:8080', 'http://localhost:8081'];
export const options = { scenarios: { s: { executor: 'constant-arrival-rate', rate: 2000, timeUnit: '1s', duration: '10m', preAllocatedVUs: 4000, maxVUs: 4000 } }, summaryTrendStats: ['med', 'p(99)', 'max'] };
let n = 0;
export default function () {
  const res = http.post(`${urls[n++ % 2]}/transfers`, '{}', { headers: { Authorization: 'Bearer local-development-key', 'Content-Type': 'application/json' }, timeout: '3s' });
  if (res.status === 0) eof.add(1);
}
