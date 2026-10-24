// Поток событий пользователей: 2 500 правок услуг и счетов на постоянной скорости 30 событий в секунду,
// параллельно читается очередь. Пороги — только на ошибки; p95 записи и чтения идут в сводку как результат.
// Перед запуском загрузите набор организаторов (cases/analizator/data.json) через PUT /source/snapshot.
// Запуск: k6 run -e PASSWORD=… docs/load/k6/events.js
import http from 'k6/http';
import { check } from 'k6';
import { Trend } from 'k6/metrics';
import { loginOnce, authHeaders, API } from './common.js';

const EVENTS = 2500;
const RATE = 30; // событий в секунду: сервер пишет снимок целиком, последовательно ≈ 30–36 событий/с (docs/load/2026-10-24-events-2500.md)

const writeMs = new Trend('event_write_ms', true);
const readMs = new Trend('queue_read_ms', true);

export const options = {
  scenarios: {
    events: {
      executor: 'constant-arrival-rate',
      rate: RATE,
      timeUnit: '1s',
      duration: `${Math.ceil(EVENTS / RATE)}s`,
      preAllocatedVUs: 20,
      maxVUs: 50,
      exec: 'event',
    },
    queue: { executor: 'constant-vus', vus: 2, duration: `${Math.ceil(EVENTS / RATE)}s`, exec: 'queue' },
  },
  thresholds: {
    http_req_failed: ['rate==0'],
    checks: ['rate==1'],
    event_write_ms: ['p(95)>=0'], // порог без условия: p95 попадает в сводку
    queue_read_ms: ['p(95)>=0'],
  },
  summaryTrendStats: ['avg', 'med', 'p(95)', 'max'],
};

// Живые услуги и счета берём из карточек клиентов, чтобы не угадывать id.
export function setup() {
  const session = loginOnce();
  const list = http.get(`${API}/customers?limit=200`, { headers: authHeaders(session) }).json('items');
  const services = [];
  const accounts = [];
  for (const row of list) {
    accounts.push(row.id);
    const card = http.get(`${API}/customers/${row.id}`, { headers: authHeaders(session) }).json();
    for (const s of card.services) {
      if (['active', 'suspended', 'prolong_wait'].includes(s.status)) services.push(s.id);
    }
  }
  return { token: session.token, services, accounts };
}

function patch(data, path, body) {
  const res = http.patch(`${API}/source/${path}`, JSON.stringify(body), {
    headers: authHeaders(data, { 'Content-Type': 'application/json' }),
  });
  writeMs.add(res.timings.duration);
  check(res, { 'правка 200': (r) => r.status === 200 });
}

// Те же события, что в TestEventStreamConsistency. Состояние услуги между вызовами не помним:
// статусы чередуются по номеру итерации, возможны только допустимые значения.
export function event(data) {
  const n = __ITER + __VU * 7919;
  const service = data.services[n % data.services.length];
  const account = data.accounts[n % data.accounts.length];
  switch (n % 6) {
    case 0:
    case 1:
      return patch(data, `accounts/${account}`, { balance: 100 * (1 + (n % 500)) });
    case 2:
      return patch(data, `services/${service}`, { autoProlong: true });
    case 3:
      return patch(data, `services/${service}`, { autoProlong: false });
    case 4:
      return patch(data, `services/${service}`, { status: n % 12 < 6 ? 'suspended' : 'active' });
    default:
      return patch(data, `services/${service}`, { status: n % 12 < 6 ? 'prolong_wait' : 'active' });
  }
}

export function queue(data) {
  const res = http.get(`${API}/customers?sort=expectedLoss&order=desc&limit=50`, { headers: authHeaders(data) });
  readMs.add(res.timings.duration);
  check(res, { 'очередь 200': (r) => r.status === 200 });
}
