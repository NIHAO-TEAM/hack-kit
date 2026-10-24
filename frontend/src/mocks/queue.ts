import { HttpResponse, http } from 'msw';
import type { components } from '@shared/api';
import data from './data.json';
import { openTaskBrief, taskFilterMatches } from './tasks';

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

/** Блок «Работа с клиентами»: считается из открытых задач моков, как на сервере. */
function retention(all: ReturnType<typeof rows>) {
  const tasks = all.map((r) => ({ r, task: openTaskBrief(r.id) }));
  const inWork = tasks.filter((t) => t.task);
  const without = (level: RiskLevel) =>
    all.filter((r) => r.level === level && !openTaskBrief(r.id));
  const loss = (list: typeof all) => list.reduce((s, r) => s + r.expectedLossCents, 0);
  const count = (status: string) => inWork.filter((t) => t.task?.status === status).length;
  return {
    tasksInWork: count('in_work'),
    tasksContacted: count('contacted'),
    tasksPostponed: count('postponed'),
    tasksOverdue: inWork.filter((t) => t.task?.overdue).length,
    customersInWork: inWork.length,
    lossInWorkCents: loss(inWork.map((t) => t.r)),
    highWithoutTask: without('high').length,
    highWithoutTaskLossCents: loss(without('high')),
    mediumWithoutTask: without('medium').length,
    mediumWithoutTaskLossCents: loss(without('medium')),
  };
}

/** Клиенты по причинам (условно) и доля их потери, приходящаяся на причину; как в `reason` сервера. */
const reasonShare: Record<string, Record<number, number>> = {
  susp: { 2: 0.6, 1: 0.3 },
  funds: { 2: 0.4 },
};

export const queueHandlers = [
  http.get(`${API}/customers`, ({ request }) => {
    const url = new URL(request.url);
    const sort = url.searchParams.get('sort') ?? 'expectedLoss';
    const dir = url.searchParams.get('order') === 'asc' ? 1 : -1;
    const level = url.searchParams.get('level');
    const q = (url.searchParams.get('q') ?? '').toLowerCase();
    const billing = url.searchParams.get('billing');
    const task = url.searchParams.get('task');
    const reason = url.searchParams.get('reason');
    if (reason && !(reason in reasonShare)) {
      return HttpResponse.json(
        { type: 'about:blank', title: 'reason: неизвестная причина', status: 400 },
        { status: 400, headers: { 'Content-Type': 'application/problem+json' } },
      );
    }
    const share = reason ? reasonShare[reason]! : null;
    const key = sortKey[sort] ?? sortKey['expectedLoss']!;
    const byReason = (r: { id: number; expectedLossCents: number }) =>
      Math.round(r.expectedLossCents * (share?.[r.id] ?? 0));
    const items = rows()
      .map((r) => ({
        ...r,
        task: openTaskBrief(r.id),
        ...(share ? { reasonLossCents: byReason(r) } : {}),
      }))
      .filter(
        (r) =>
          (!share || r.id in share) &&
          (!level || r.level === level) &&
          r.email.toLowerCase().includes(q) &&
          taskFilterMatches(task, r.task) &&
          (!billing || Boolean(r.billing) === (billing === 'only')),
      )
      .sort((a, b) => {
        if (share && !url.searchParams.has('sort'))
          return (byReason(b) - byReason(a)) * -dir || a.id - b.id;
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
      lossByFeature: [
        { featureId: 'susp', lossCents: 184419, customers: 2, sharePercent: 51.24 },
        { featureId: 'funds', lossCents: 134738, customers: 1, sharePercent: 37.44 },
        { featureId: 'noauto', lossCents: 38321, customers: 5, sharePercent: 10.65 },
        { featureId: 'pwait', lossCents: 2003, customers: 2, sharePercent: 0.56 },
        { featureId: 'fail', lossCents: 426, customers: 3, sharePercent: 0.12 },
      ],
      asOf: MOCK_AS_OF,
      retention: retention(all),
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
