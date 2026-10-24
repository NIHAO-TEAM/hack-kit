// Приём событий извне: POST /api/v1/source/events пачками по 500 событий на постоянной скорости
// (по умолчанию 10 пачек в секунду = 5 000 событий/с, 60 с). Параллельно читается очередь клиентов.
// 202 — норма, 503 с Retry-After — обратное давление (считается отдельно, не ошибка); всё остальное — ошибка.
// Перед запуском загрузите снимок (PUT /source/snapshot). Токен источника не нужен: входим как аналитик.
// Запуск: k6 run -e PASSWORD=… [-e BATCH_RATE=10 -e DURATION=60s] docs/load/k6/ingest.js
import http from 'k6/http';
import { check } from 'k6';
import { Counter, Trend } from 'k6/metrics';
import { loginOnce, authHeaders, API } from './common.js';

const BATCH = 500;
const RATE = Number(__ENV.BATCH_RATE || 10);
const DURATION = __ENV.DURATION || '60s';

const ingestMs = new Trend('ingest_batch_ms', true);
const readMs = new Trend('queue_read_ms', true);
const sent = new Counter('ingest_events_accepted');
const dups = new Counter('ingest_events_duplicate');
const shed = new Counter('ingest_batches_shed');

http.setResponseCallback(http.expectedStatuses(200, 202, 503));

export const options = {
  scenarios: {
    ingest: {
      executor: 'constant-arrival-rate',
      rate: RATE,
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: 20,
      maxVUs: 100,
      exec: 'ingest',
    },
    queue: { executor: 'constant-vus', vus: 2, duration: DURATION, exec: 'queue' },
  },
  thresholds: {
    http_req_failed: ['rate==0'],
    checks: ['rate==1'],
    ingest_batch_ms: ['p(95)>=0'], // порог без условия: p95 попадает в сводку
    queue_read_ms: ['p(95)>=0'],
  },
  summaryTrendStats: ['avg', 'med', 'p(95)', 'max'],
};

// Счета берём из списка клиентов (id клиента = id счёта), чтобы не угадывать номера.
export function setup() {
  const session = loginOnce();
  const list = http.get(`${API}/customers?limit=200`, { headers: authHeaders(session) }).json('items');
  return { token: session.token, accounts: list.map((r) => r.id), run: Date.now().toString(36) };
}

export function ingest(data) {
  const lines = [];
  for (let i = 0; i < BATCH; i++) {
    const n = __ITER * BATCH + i + __VU * 1000003;
    lines.push(
      JSON.stringify({
        eventId: `${data.run}-${__VU}-${__ITER}-${i}`,
        kind: 'top_up',
        accountId: data.accounts[n % data.accounts.length],
        balance: 100 * (1 + (n % 500)),
      }),
    );
  }
  const res = http.post(`${API}/source/events`, lines.join('\n'), {
    headers: authHeaders(data, { 'Content-Type': 'application/x-ndjson' }),
  });
  ingestMs.add(res.timings.duration);
  if (res.status === 503) {
    shed.add(1);
    check(res, { '503 с Retry-After': (r) => !!r.headers['Retry-After'] });
    return;
  }
  const ok = check(res, { 'пачка принята (202)': (r) => r.status === 202 });
  if (ok) {
    sent.add(res.json('accepted'));
    dups.add(res.json('duplicate'));
    check(res, { 'без rejected': (r) => r.json('rejected').length === 0 });
  }
}

export function queue(data) {
  const res = http.get(`${API}/customers?sort=expectedLoss&order=desc&limit=50`, { headers: authHeaders(data) });
  readMs.add(res.timings.duration);
  check(res, { 'очередь 200': (r) => r.status === 200 });
}
