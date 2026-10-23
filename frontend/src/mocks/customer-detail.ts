import { HttpResponse, http } from 'msw';
import type { components } from '@shared/api';

type Detail = components['schemas']['CustomerDetail'];

/** Карточки двух ключевых клиентов демо-сценария; остальные собирает `fallback`. */
const featured: Record<number, Pick<Detail, 'features' | 'actions' | 'explanation'>> = {
  9: {
    features: [
      { id: 'funds', value: 1, weight: 0.7, contribution: 0.7, reason: 'Баланса не хватит' },
      { id: 'susp', value: 1, weight: 0.35, contribution: 0.35, reason: 'Услуга приостановлена' },
      {
        id: 'noauto',
        value: 1,
        weight: 0.55,
        contribution: 0.55,
        reason: 'Автопродление выключено',
      },
    ],
    actions: [
      {
        id: 'topup',
        title: 'Пополнить на 529,84 €',
        effectCents: -106748,
        newScore: 0.2,
        newLevel: 'low',
      },
      {
        id: 'autoprolong',
        title: 'Включить автопродление',
        effectCents: -41000,
        newScore: 0.5,
        newLevel: 'medium',
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
        customer: { ...base.customer, score: 0.87, level: 'high', expectedLossCents: 345927 },
      });
    }
    if (id === 2) {
      return HttpResponse.json({
        ...base,
        customer: { ...base.customer, level: 'lost', score: null, expectedLossCents: 0 },
        explanation: 'Уже потеряно: все услуги заблокированы',
      });
    }
    return HttpResponse.json(extra ? { ...base, ...extra } : base);
  });
}
