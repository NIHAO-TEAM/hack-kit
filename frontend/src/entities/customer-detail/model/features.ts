import type { CustomerFeature, ServiceRisk } from '../api/customer';

/** Признаки по убыванию вклада: сначала то, что сильнее всего двигает оценку. */
export function byContribution(features: readonly CustomerFeature[]): CustomerFeature[] {
  return [...features].sort((a, b) => b.contribution - a.contribution);
}

/** Скор в шапке: целые проценты, как в очереди. Сервер отдаёт `score` в процентах 0..100. */
export function scorePercentText(score: number): string {
  return `скор ${Math.round(score)} %`;
}

/** Формула вероятности срыва с числами ответа: `P = 1 − (1 − w·f)(…) = 0,87`. Скор с сервера — в процентах, в формуле вероятность 0..1. */
export function formulaText(features: readonly CustomerFeature[], scorePercent: number): string {
  const num = (n: number) => String(Math.round(n * 100) / 100).replace('.', ',');
  const terms = byContribution(features).map((f) => `(1 − ${num(f.weight)}·${num(f.value)})`);
  return `P = 1 − ${terms.join(' · ') || '1'} = ${num(scorePercent / 100)}`;
}

/** Сумма ожидаемых потерь по услугам; на сервере ожидаемая потеря клиента — ровно эта сумма (`risk/score.go`). */
export function lossSum(risks: readonly ServiceRisk[]): number {
  return risks.reduce((sum, r) => sum + r.lossCents, 0);
}
