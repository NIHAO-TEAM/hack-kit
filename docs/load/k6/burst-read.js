// Чтение под залпом событий: 20 VU читают очередь, 2 VU — сводку. Идёт, пока burst.sh не пошлёт SIGINT.
// Запуск: k6 run -e PASSWORD=… docs/load/k6/burst-read.js (обычно его запускает docs/load/burst.sh)
import http from 'k6/http';
import { check } from 'k6';
import { Trend } from 'k6/metrics';
import { loginOnce, authHeaders, API } from './common.js';

const queueMs = new Trend('queue_read_ms', true);

export const options = {
  scenarios: {
    queue: { executor: 'constant-vus', vus: 20, duration: '2h', exec: 'queue' },
    summary: { executor: 'constant-vus', vus: 2, duration: '2h', exec: 'summary' },
  },
  thresholds: { http_req_failed: ['rate==0'], queue_read_ms: ['p(95)>=0'] }, // p95 попадает в сводку
  summaryTrendStats: ['avg', 'med', 'p(95)', 'max'],
};

export function setup() {
  return loginOnce();
}

export function queue(session) {
  const res = http.get(`${API}/customers?sort=expectedLoss&order=desc&limit=50&offset=${(__ITER % 20) * 50}`, { headers: authHeaders(session) });
  queueMs.add(res.timings.duration);
  check(res, { 'очередь 200': (r) => r.status === 200 });
}

export function summary(session) {
  const res = http.get(`${API}/analytics/summary`, { headers: authHeaders(session) });
  check(res, { 'summary 200': (r) => r.status === 200 });
}
