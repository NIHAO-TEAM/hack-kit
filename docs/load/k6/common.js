import http from 'k6/http';
import { check, fail } from 'k6';

export const BASE = __ENV.BASE_URL || 'http://localhost:8080';
export const API = `${BASE}/api/v1`;

// Вход аналитиком; сессия живёт в cookie jar виртуального пользователя.
export function login() {
  const res = http.post(
    `${API}/auth/login`,
    JSON.stringify({ login: __ENV.LOGIN || 'admin', password: __ENV.PASSWORD }),
    { headers: { 'Content-Type': 'application/json' } },
  );
  if (!check(res, { 'вход 200': (r) => r.status === 200 })) {
    fail(`вход не удался: ${res.status}`);
  }
}
