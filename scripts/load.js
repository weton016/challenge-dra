import http from 'k6/http';
import { check } from 'k6';
import exec from 'k6/execution';
import { Counter } from 'k6/metrics';

const http200 = new Counter('status_200');
const http503 = new Counter('status_503');
const httpOther = new Counter('status_other');

const rate = Number(__ENV.RATE || 2000);
const coldRate = Number(__ENV.COLD_RATE || 0);
const duration = __ENV.DURATION || '10m';
const prefix = __ENV.RUN_ID || `load-${Date.now()}`;
const urls = (__ENV.URLS || 'http://localhost:8080,http://localhost:8081').split(',');
const key = __ENV.API_KEY || 'local-development-key';
const scenarios = {
  hot: { executor: 'constant-arrival-rate', exec: 'hot', rate, timeUnit: '1s', duration, preAllocatedVUs: Math.max(rate * 2, 100), maxVUs: Math.max(rate * 2, 100) },
};
if (coldRate > 0) {
  scenarios.cold = { executor: 'constant-arrival-rate', exec: 'cold', rate: coldRate, timeUnit: '1s', duration, preAllocatedVUs: Math.max(coldRate * 4, 50), maxVUs: Math.max(coldRate * 4, 50) };
}
export const options = {
  scenarios,
  thresholds: { http_req_duration: ['p(99)<300'], http_req_failed: ['rate==0'], checks: ['rate==1'], dropped_iterations: ['count==0'] },
  summaryTrendStats: ['avg', 'med', 'p(95)', 'p(99)', 'max'],
};

// RFC 4122 shaped: run start in ms, scenario nibble, iteration number.
const run = Date.now().toString(16).padStart(12, '0').slice(-12);
const scenarioNibble = { in: '1', out: '2', mixed: '3', cold: '4' };
function uuid(scenario, n) {
  const h = n.toString(16).padStart(17, '0');
  return `${run.slice(0, 8)}-${run.slice(8)}-4${scenarioNibble[scenario]}${h.slice(0, 2)}-8${h.slice(2, 5)}-${h.slice(5)}`;
}

function post(source, destination, scenario, n) {
  const body = { transaction_id: uuid(scenario, n), source_id: source, destination_id: destination, amount_minor: 1 };
  const res = http.post(`${urls[n % urls.length]}/transfers`, JSON.stringify(body), { headers: { Authorization: `Bearer ${key}`, 'Content-Type': 'application/json' }, timeout: '3s' });
  if (res.status === 200) http200.add(1);
  else if (res.status === 503) http503.add(1);
  else httpOther.add(1);
  check(res, { approved: (r) => r.status === 200 && r.json('decision') === 'APPROVED' });
}

export function hot() {
  const n = exec.scenario.iterationInTest;
  const peer = 2 + (n % 999998);
  const mode = __ENV.MODE || 'mixed';
  const incoming = mode === 'in' || (mode === 'mixed' && n % 2 === 0);
  const source = incoming ? peer : 1;
  const destination = incoming ? 1 : peer;
  post(source, destination, mode, n);
}

export function cold() {
  const n = exec.scenario.iterationInTest;
  post(10000 + (n % 400000), 500000 + (n % 400000), 'cold', n);
}

export function handleSummary(data) {
  return { ['reports/load-' + prefix + '.json']: JSON.stringify(data, null, 2) };
}
