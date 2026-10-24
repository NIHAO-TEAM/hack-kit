// 500 SSE-клиентов держат GET /events. k6 не отдаёт тело потока по частям, поэтому клиент живёт окнами
// по WINDOW секунд: окно закончилось по таймауту — поток был жив всё окно; ответ пришёл раньше — обрыв
// (sse_drops), клиент тут же переподключается следующей итерацией. Идёт, пока burst.sh не пошлёт SIGINT.
// Запуск: k6 run -e PASSWORD=… docs/load/k6/sse.js
import http from 'k6/http';
import { Counter, Rate } from 'k6/metrics';
import { loginOnce, authHeaders, API } from './common.js';

const WINDOW = Number(__ENV.WINDOW || 10);
const drops = new Counter('sse_drops');
const alive = new Rate('sse_window_alive');

export const options = {
  scenarios: { sse: { executor: 'constant-vus', vus: Number(__ENV.CLIENTS || 500), duration: '2h' } },
  thresholds: { sse_drops: ['count==0'] },
};

export function setup() {
  return loginOnce();
}

export default function (session) {
  const res = http.get(`${API}/events`, {
    headers: authHeaders(session, { Accept: 'text/event-stream' }),
    timeout: `${WINDOW}s`,
    responseType: 'none',
  });
  const timedOut = res.error_code === 1050; // k6: request timeout — ожидаемый конец окна
  alive.add(timedOut);
  if (!timedOut) drops.add(1);
}
