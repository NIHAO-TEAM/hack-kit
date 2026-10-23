// PATCH /model/settings на 100k: время до ответа и до события recalculated.
// Цель: ≤ 1 с. Запуск: k6 run -e PASSWORD=… -e METRICS_TOKEN=… docs/load/k6/recalc.js
import http from 'k6/http';
import { check } from 'k6';
import { Trend } from 'k6/metrics';
import { login, API, BASE } from './common.js';

const patchSeconds = new Trend('patch_settings_seconds');
const recalcGauge = new Trend('server_recalc_seconds');

export const options = {
  vus: 1,
  iterations: 10,
  thresholds: { patch_settings_seconds: ['p(95)<1'] },
};

export function setup() {
  login();
}

export default function () {
  if (__ITER === 0) login();
  // Горизонт чередуем, чтобы каждый PATCH менял настройки и вызывал пересчёт.
  const horizonDays = __ITER % 2 === 0 ? 30 : 60;
  const res = http.patch(`${API}/model/settings`, JSON.stringify({ horizonDays }), {
    headers: { 'Content-Type': 'application/json' },
    timeout: '30s',
  });
  patchSeconds.add(res.timings.duration / 1000);
  check(res, { 'настройки 200': (r) => r.status === 200 });

  // Время пересчёта по версии сервера — из churn_recalc_duration_seconds.
  const m = http.get(`${BASE}/metrics`, { headers: { Authorization: `Bearer ${__ENV.METRICS_TOKEN}` } });
  const hit = /^churn_recalc_duration_seconds\s+([0-9.eE+-]+)/m.exec(m.body || '');
  if (hit) recalcGauge.add(parseFloat(hit[1]));
}
