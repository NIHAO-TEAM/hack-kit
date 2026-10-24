import { HttpResponse, http } from 'msw';
import type { components } from '@shared/api';
import { openTaskBrief } from './tasks';

type Detail = components['schemas']['CustomerDetail'];

/** Карточки двух ключевых клиентов демо-сценария; остальные собирает `fallback`. */
const featured: Record<
  number,
  Pick<Detail, 'features' | 'actions' | 'explanation' | 'serviceRisks'>
> = {
  // Числа — из testdata/golden/organizers.json, клиент 9 (признаки услуги 9).
  9: {
    features: [
      {
        id: 'noauto',
        value: 1,
        weight: 0.55,
        contribution: 0.55,
        reason: 'Автопродление выключено',
      },
      { id: 'fail', value: 1, weight: 0.4, contribution: 0.4, reason: 'Были неудачные платежи' },
      { id: 'susp', value: 1, weight: 0.35, contribution: 0.35, reason: 'Услуга приостановлена' },
      {
        id: 'funds',
        value: 0.366697,
        weight: 0.7,
        contribution: 0.256688,
        reason: 'Баланса не хватит',
      },
      {
        id: 'flight',
        value: 0.605337,
        weight: 0,
        contribution: 0,
        reason: 'Часть баланса доступна к выводу. Факт вывода в данных не виден',
      },
    ],
    actions: [
      {
        id: 'topup',
        title: 'Пополнить на 529,84 €',
        effectCents: -102749,
        newScore: 0.8245,
        newLevel: 'high',
      },
      {
        id: 'autoprolong',
        title: 'Включить автопродление',
        effectCents: -123,
        newScore: 0.7364,
        newLevel: 'high',
      },
    ],
    explanation: 'Платежи на 30 дней × вероятность срыва',
    // Оценки услуг 9, 19, 29 — расчёт по testdata/golden/organizers.json (веса из настроек, R и P·R
    // сходятся с ожидаемой потерей клиента 318 910).
    // R: golden даёт только Σ R = 616 450 (`exposureCents`) и признаки; R по услугам — из прайса `data.json`
    // и числа списаний за 720 ч (`risk/charges.go`): 850 + 615 600 + 0 (полугодовая услуга 29 в горизонт не продлевается).
    serviceRisks: [
      {
        serviceId: 9,
        probability: 0.869549,
        rCents: 850,
        lossCents: 739,
        features: [
          {
            id: 'noauto',
            value: 1,
            weight: 0.55,
            contribution: 0.55,
            reason: 'Автопродление выключено',
          },
          {
            id: 'fail',
            value: 1,
            weight: 0.4,
            contribution: 0.4,
            reason: 'Были неудачные платежи',
          },
          {
            id: 'susp',
            value: 1,
            weight: 0.35,
            contribution: 0.35,
            reason: 'Услуга приостановлена',
          },
          {
            id: 'funds',
            value: 0.366697,
            weight: 0.7,
            contribution: 0.256688,
            reason: 'Баланса не хватит',
          },
          {
            id: 'flight',
            value: 0.605337,
            weight: 0,
            contribution: 0,
            reason: 'Часть баланса доступна к выводу. Факт вывода в данных не виден',
          },
        ],
        quality: [],
      },
      {
        serviceId: 19,
        probability: 0.516847,
        rCents: 615600,
        lossCents: 318171,
        features: [
          {
            id: 'funds',
            value: 0.366697,
            weight: 0.7,
            contribution: 0.256688,
            reason: 'Баланса не хватит',
          },
          {
            id: 'susp',
            value: 1,
            weight: 0.35,
            contribution: 0.35,
            reason: 'Услуга приостановлена',
          },
          {
            id: 'flight',
            value: 0.605337,
            weight: 0,
            contribution: 0,
            reason: 'Часть баланса доступна к выводу. Факт вывода в данных не виден',
          },
        ],
        quality: [],
      },
      {
        serviceId: 29,
        probability: 0.516847,
        rCents: 0,
        lossCents: 0,
        features: [
          {
            id: 'funds',
            value: 0.366697,
            weight: 0.7,
            contribution: 0.256688,
            reason: 'Баланса не хватит',
          },
          {
            id: 'susp',
            value: 1,
            weight: 0.35,
            contribution: 0.35,
            reason: 'Услуга приостановлена',
          },
          {
            id: 'flight',
            value: 0.605337,
            weight: 0,
            contribution: 0,
            reason: 'Часть баланса доступна к выводу. Факт вывода в данных не виден',
          },
        ],
        quality: [],
      },
    ],
  },
};

export function customerDetailHandler(
  fallback: (id: number) => Detail | undefined,
  authorized: () => boolean,
  problem: (status: number, title: string) => Response,
) {
  return http.get('/api/v1/customers/:id', ({ params }) => {
    if (!authorized()) return problem(401, 'Нет сессии');
    const id = Number(params.id);
    const base = fallback(id);
    if (!base) return problem(404, 'Клиент не найден');
    const extra = featured[id];
    const task = openTaskBrief(id);
    const detail = extra ? { ...base, ...extra } : base;
    const withTask = (d: Detail): Detail => ({ ...d, customer: { ...d.customer, task } });
    if (id === 9) {
      return HttpResponse.json(
        withTask({
          ...detail,
          customer: { ...base.customer, score: 86.95, level: 'high', expectedLossCents: 318910 },
        }),
      );
    }
    return HttpResponse.json(withTask(detail));
  });
}
