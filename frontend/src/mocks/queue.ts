import { HttpResponse, http } from 'msw';
import type { components } from '@shared/api';
import data from './data.json';

type CustomerRow = components['schemas']['CustomerRow'];
type RiskLevel = components['schemas']['RiskLevel'];
type Account = components['schemas']['Account'];

const API = '/api/v1';
const HOUR = 3_600_000;

/** Время стенда в моках: фиксировано, чтобы скриншоты и тесты были повторяемы. */
export const MOCK_AS_OF = '2026-10-19T15:00:00.000Z';
const NOW = Date.parse(MOCK_AS_OF);
const at = (hoursFromNow: number) => new Date(NOW + hoursFromNow * HOUR).toISOString();

/** Оценки очереди условные: набор покрывает все виды строк (окно, закрыто, без оценки, потерян). */
const scenario: Array<Partial<CustomerRow> & { id: number }> = [
  {
    id: 1,
    level: 'medium',
    score: 56,
    expectedLossCents: 37508,
    expiresAt: at(0.5),
    topReason: 'услуга закончится через 30 минут, автопродления нет',
    topAction: { id: 'mail', title: 'Письмо', effectCents: -37508 },
  },
  {
    id: 2,
    level: 'high',
    score: 88,
    expectedLossCents: 318910,
    expiresAt: at(-60),
    topReason: 'баланса хватает на 4 дня, при этом ожидает оплаты 3 услуги',
    topAction: {
      id: 'topup',
      title: 'Пополнить',
      effectCents: -102749,
    },
  },
  {
    id: 3,
    level: 'high',
    score: 91,
    expectedLossCents: 1472,
    expiresAt: at(-200),
    billing: true,
    topReason: 'окно закрыто, статус устарел',
  },
  {
    id: 4,
    level: 'medium',
    score: 61,
    expectedLossCents: 706,
    expiresAt: at(80),
    topReason: 'одна услуга без автопродления',
  },
  { id: 5, level: 'low', score: 12, expectedLossCents: 0, topReason: 'рисков не найдено' },
  { id: 6, level: 'unscored', score: null, expectedLossCents: 0 },
  {
    id: 7,
    level: 'lost',
    score: null,
    expectedLossCents: 0,
    lostCents: 680874,
    topReason: 'услуги удалены биллингом',
  },
];

export function rows(): CustomerRow[] {
  const accounts = new Map((data.accounts as Account[]).map((a) => [a.id, a]));
  return scenario.map((s) => {
    const a = accounts.get(s.id) ?? (data.accounts[0] as Account);
    return {
      email: a.email.replace(/\d{4}/, String(s.id).padStart(4, '0')),
      balance: a.balance,
      totalReplenished: a.totalReplenished,
      score: null,
      ...s,
    } as CustomerRow;
  });
}

const bottom: RiskLevel[] = ['unscored', 'lost'];
const sortKey: Record<string, (r: CustomerRow) => number | string> = {
  id: (r) => r.id,
  email: (r) => r.email,
  balance: (r) => r.balance,
  totalReplenished: (r) => r.totalReplenished,
  score: (r) => r.score ?? -1,
  expectedLoss: (r) => r.expectedLossCents,
};

export const queueHandlers = [
  http.get(`${API}/customers`, ({ request }) => {
    const url = new URL(request.url);
    const sort = url.searchParams.get('sort') ?? 'expectedLoss';
    const dir = url.searchParams.get('order') === 'asc' ? 1 : -1;
    const level = url.searchParams.get('level');
    const q = (url.searchParams.get('q') ?? '').toLowerCase();
    const billing = url.searchParams.get('billing');
    const key = sortKey[sort] ?? sortKey['expectedLoss']!;
    const items = rows()
      .filter(
        (r) =>
          (!level || r.level === level) &&
          r.email.toLowerCase().includes(q) &&
          (!billing || Boolean(r.billing) === (billing === 'only')),
      )
      .sort((a, b) => {
        const lowA = bottom.includes(a.level);
        const lowB = bottom.includes(b.level);
        if (lowA !== lowB) return lowA ? 1 : -1;
        const ka = key(a);
        const kb = key(b);
        return (ka < kb ? -1 : ka > kb ? 1 : 0) * dir || a.id - b.id;
      });
    const limit = Number(url.searchParams.get('limit') ?? 200);
    const offset = Number(url.searchParams.get('offset') ?? 0);
    return HttpResponse.json({ items: items.slice(offset, offset + limit), total: items.length });
  }),
  http.get(`${API}/analytics/summary`, () => {
    const all = rows();
    const count = (l: RiskLevel) => all.filter((r) => r.level === l).length;
    return HttpResponse.json({
      levels: {
        high: count('high'),
        medium: count('medium'),
        low: count('low'),
        unscored: count('unscored'),
        lost: count('lost'),
      },
      expectedLossCents: all.reduce((s, r) => s + r.expectedLossCents, 0),
      atRiskCents: 359907,
      lostCents: 120450,
      asOf: MOCK_AS_OF,
      version: 1,
    });
  }),
  http.get(`${API}/analytics/churn-chart`, () => {
    const actual = [0, 1, 0, 2, 1, 1, 0, 1, 1];
    const forecast = [1, 2, 1, 3, 2, 1, 2, 1];
    const week = 7 * 86_400_000;
    const monday = Date.parse('2026-08-24T00:00:00Z');
    const points = Array.from({ length: 17 }, (_, i) => ({
      weekStart: new Date(monday + i * week).toISOString().slice(0, 10),
      actual: i <= 8 ? (actual[i] ?? 0) : null,
      forecast: i >= 8 ? (forecast[i - 8] ?? 0) : null,
    }));
    return HttpResponse.json({ points });
  }),
];
