import { HttpResponse, http } from 'msw';
import type { components } from '@shared/api';

type Detail = components['schemas']['CustomerDetail'];

/** Карточки двух ключевых клиентов демо-сценария; остальные собирает `fallback`. */
const featured: Record<number, Pick<Detail, 'features' | 'actions' | 'explanation'>> = {
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
        weight: 0.15,
        contribution: 0.0908,
        reason: 'Баланс уходит',
      },
    ],
    actions: [
      {
        id: 'topup',
        title: 'Пополнить на 529,84 €',
        effectCents: -106748,
        newScore: 0.8346,
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
    if (id === 9) {
      return HttpResponse.json({
        ...base,
        ...extra,
        customer: { ...base.customer, score: 88.14, level: 'high', expectedLossCents: 345927 },
      });
    }
    return HttpResponse.json(extra ? { ...base, ...extra } : base);
  });
}
