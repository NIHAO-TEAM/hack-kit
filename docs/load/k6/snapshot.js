// PUT /source/snapshot на 100k клиентов. Цель: ≤ 30 с.
// Запуск: k6 run -e SOURCE_TOKEN=… -e SNAPSHOT=/path/data.json docs/load/k6/snapshot.js
import http from 'k6/http';
import { check } from 'k6';
import { Trend } from 'k6/metrics';
import { API } from './common.js';

const body = open(__ENV.SNAPSHOT || 'data.json');
const putSeconds = new Trend('snapshot_put_seconds');

export const options = {
  vus: 1,
  iterations: 3,
  thresholds: { snapshot_put_seconds: ['max<30'] },
};

export default function () {
  const res = http.put(`${API}/source/snapshot`, body, {
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${__ENV.SOURCE_TOKEN}` },
    timeout: '120s',
  });
  putSeconds.add(res.timings.duration / 1000);
  check(res, { 'снимок принят': (r) => r.status === 200 });
}
