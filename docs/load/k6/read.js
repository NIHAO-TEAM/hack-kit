// Чтение: список с сортировкой и summary. Цель: p95 списка — записать в README.
// Запуск: k6 run -e PASSWORD=… docs/load/k6/read.js
import http from 'k6/http';
import { check } from 'k6';
import { login, API } from './common.js';

const sorts = ['expectedLoss', 'score', 'balance', 'totalReplenished', 'email', 'id'];

export const options = {
  scenarios: {
    list: { executor: 'constant-vus', vus: 10, duration: '30s', exec: 'list' },
    summary: { executor: 'constant-vus', vus: 5, duration: '30s', exec: 'summary' },
  },
  thresholds: {
    'http_req_duration{scenario:list}': ['p(95)<1000'],
    'http_req_failed': ['rate<0.001'],
  },
};

export function list() {
  if (__ITER === 0) login();
  const sort = sorts[__ITER % sorts.length];
  const offset = (__ITER % 50) * 50;
  const res = http.get(`${API}/customers?sort=${sort}&order=desc&limit=50&offset=${offset}`);
  check(res, { 'список 200': (r) => r.status === 200 });
}

export function summary() {
  if (__ITER === 0) login();
  const res = http.get(`${API}/analytics/summary`);
  check(res, { 'summary 200': (r) => r.status === 200 });
}
