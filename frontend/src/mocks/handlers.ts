import { HttpResponse, http } from 'msw';
import type { components } from '@shared/api';
import data from './data.json';
import { queueHandlers } from './queue';

type User = components['schemas']['User'];
type CustomerRow = components['schemas']['CustomerRow'];
type RiskLevel = components['schemas']['RiskLevel'];
type Account = components['schemas']['Account'];
type Service = components['schemas']['Service'];

const API = '/api/v1';

/** Учётки моков. Пароль `demo` существует только здесь. */
const users: Record<string, User> = {
  marketer: { id: 1, login: 'marketer', role: 'marketer' },
  analyst: { id: 2, login: 'analyst', role: 'analyst' },
  jury: { id: 3, login: 'jury', role: 'jury' },
};

const SESSION_KEY = 'mock-session';

/** Сессия моков живёт в sessionStorage: перезагрузка вкладки её не сбрасывает. */
function getCurrent(): User | null {
  const login = sessionStorage.getItem(SESSION_KEY);
  return login ? (users[login] ?? null) : null;
}

function setCurrent(user: User | null): void {
  if (user) sessionStorage.setItem(SESSION_KEY, user.login);
  else sessionStorage.removeItem(SESSION_KEY);
}

function problem(status: number, title: string, headers?: Record<string, string>) {
  return HttpResponse.json(
    { type: 'about:blank', title, status },
    { status, headers: { 'Content-Type': 'application/problem+json', ...headers } },
  );
}

const accounts = data.accounts as Account[];
const services = data.services as Service[];

function levelOf(score: number): RiskLevel {
  if (score >= 0.7) return 'high';
  if (score >= 0.4) return 'medium';
  return 'low';
}

/** Оценки моков условные: детерминированы по id и нужны только для вёрстки. */
function rows(): CustomerRow[] {
  return accounts.map((a) => {
    const score = ((a.id * 37) % 100) / 100;
    return {
      id: a.id,
      email: a.email,
      balance: a.balance,
      totalReplenished: a.totalReplenished,
      score,
      level: levelOf(score),
      expectedLossCents: Math.round(a.totalReplenished * score),
    };
  });
}

export const handlers = [
  ...queueHandlers,
  http.post(`${API}/auth/login`, async ({ request }) => {
    const body = (await request.json()) as { login: string; password: string };
    if (body.login === 'toomany') {
      return problem(429, 'Слишком много попыток входа', { 'Retry-After': '300' });
    }
    const user = users[body.login];
    if (!user || body.password !== 'demo') return problem(401, 'Неверный логин или пароль');
    setCurrent(user);
    return HttpResponse.json({ user, csrfToken: 'mock-csrf' });
  }),
  http.get(`${API}/auth/me`, () => {
    const current = getCurrent();
    return current ? HttpResponse.json(current) : problem(401, 'Нет сессии');
  }),
  http.post(`${API}/auth/logout`, () => {
    setCurrent(null);
    return new HttpResponse(null, { status: 204 });
  }),
  http.get(`${API}/customers`, () => {
    if (!getCurrent()) return problem(401, 'Нет сессии');
    const items = rows().sort((a, b) => b.expectedLossCents - a.expectedLossCents);
    return HttpResponse.json({ items, total: items.length });
  }),
  http.get(`${API}/customers/:id`, ({ params }) => {
    if (!getCurrent()) return problem(401, 'Нет сессии');
    const id = Number(params.id);
    const customer = rows().find((r) => r.id === id);
    if (!customer) return problem(404, 'Клиент не найден');
    return HttpResponse.json({
      customer,
      account: accounts.find((a) => a.id === id),
      services: services.filter((s) => s.ownerId === id),
      features: [],
      actions: [],
    });
  }),
  http.get(`${API}/analytics/summary`, () => {
    if (!getCurrent()) return problem(401, 'Нет сессии');
    const all = rows();
    const count = (l: RiskLevel) => all.filter((r) => r.level === l).length;
    return HttpResponse.json({
      levels: {
        high: count('high'),
        medium: count('medium'),
        low: count('low'),
        unscored: 0,
        lost: 0,
      },
      expectedLossCents: all.reduce((s, r) => s + r.expectedLossCents, 0),
      atRiskCents: all.filter((r) => r.level !== 'low').reduce((s, r) => s + r.balance, 0),
      lostCents: 0,
      version: 1,
    });
  }),
  http.get(`${API}/model/settings`, () => {
    if (!getCurrent()) return problem(401, 'Нет сессии');
    return HttpResponse.json({
      weights: {},
      thresholds: { high: 0.7, medium: 0.4 },
      horizonDays: 30,
      priceBasis: 'per_month',
      asOfMode: 'snapshot',
      version: 1,
    });
  }),
];
