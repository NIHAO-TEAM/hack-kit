import type { CustomerFeature } from '../api/customer';

/** Признаки по убыванию вклада: сначала то, что сильнее всего двигает оценку. */
export function byContribution(features: readonly CustomerFeature[]): CustomerFeature[] {
  return [...features].sort((a, b) => b.contribution - a.contribution);
}

/** Формула вероятности срыва с числами ответа: `P = 1 − (1 − w·f)(…) = 0,87`. Сама вероятность — с сервера. */
export function formulaText(features: readonly CustomerFeature[], score: number): string {
  const num = (n: number) => String(Math.round(n * 100) / 100).replace('.', ',');
  const terms = byContribution(features).map((f) => `(1 − ${num(f.weight)}·${num(f.value)})`);
  return `P = 1 − ${terms.join(' · ') || '1'} = ${num(score)}`;
}
