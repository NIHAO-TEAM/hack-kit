// Чтение под залпом событий: 20 VU читают очередь, 2 VU — сводку. Идёт, пока burst.sh не пошлёт SIGINT.
// Запуск: k6 run -e PASSWORD=… docs/load/k6/burst-read.js (обычно его запускает docs/load/burst.sh)
import http from 'k6/http';
import { check } from 'k6';
import { Trend, Counter } from 'k6/metrics';
import { loginOnce, authHeaders, API } from './common.js';

const queueMs = new Trend('queue_read_ms', true);

// Ошибки чтения по коду ответа (0 — оборван запрос: сброс соединения, таймаут). chaos.sh печатает их в лог шага.
const errByStatus = {
  '0': new Counter('read_err_0'),
  '401': new Counter('read_err_401'),
  '502': new Counter('read_err_502'),
  '503': new Counter('read_err_503'),
  '504': new Counter('read_err_504'),
  other: new Counter('read_err_other'),
};
let logged = 0;
let lastLoggedSecond = 0;
// noteError считает ошибку по коду и печатает первые 10 с адресом и текстом: по ним видно, кто ответил (Traefik или под).
function noteError(res, what) {
  if (res.status === 200) return;
  (errByStatus[String(res.status)] || errByStatus.other).add(1);
  // первые 10 ошибок и по одной за каждую секунду (до 120): по ним видно, когда ошибки шли
  const sec = Math.floor(Date.now() / 1000);
  if (logged < 130 && (logged < 10 || sec !== lastLoggedSecond)) {
    logged++;
    lastLoggedSecond = sec;
    console.error(`${new Date().toISOString()} ошибка чтения ${what}: status=${res.status} error=${res.error || ''} body=${String(res.body || '').slice(0, 120)}`);
  }
}

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
  noteError(res, 'очереди');
  check(res, { 'очередь 200': (r) => r.status === 200 });
}

export function summary(session) {
  const res = http.get(`${API}/analytics/summary`, { headers: authHeaders(session) });
  noteError(res, 'сводки');
  check(res, { 'summary 200': (r) => r.status === 200 });
}
