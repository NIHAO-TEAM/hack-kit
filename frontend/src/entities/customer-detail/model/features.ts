import type { CustomerFeature, ServiceRisk } from '../api/customer';

/** Признаки по убыванию вклада: сначала то, что сильнее всего двигает оценку. */
export function byContribution(features: readonly CustomerFeature[]): CustomerFeature[] {
  return [...features].sort((a, b) => b.contribution - a.contribution);
}

/** Скор в шапке: целые проценты, как в очереди. Сервер отдаёт `score` в процентах 0..100. */
export function scorePercentText(score: number): string {
  return `скор ${Math.round(score)} %`;
}

/** Сумма ожидаемых потерь по услугам; на сервере ожидаемая потеря клиента — ровно эта сумма (`risk/score.go`). */
export function lossSum(risks: readonly ServiceRisk[]): number {
  return risks.reduce((sum, r) => sum + r.lossCents, 0);
}

export type ReasonStrength = 'strong' | 'noticeable' | 'weak';

export type RiskReason = {
  id: string;
  text: string;
  strength: ReasonStrength;
};

export type ServiceReasons = {
  serviceId: number;
  lossCents: number;
  /** Доля услуги в потере клиента, 0..1: ширина полосы. */
  share: number;
  reasons: RiskReason[];
};

/** Пороги силы причины по доле её вклада в сумме вкладов: на глаз, не расчёт [ПРЕДПОЛОЖЕНО]. */
const STRONG_SHARE = 0.4;
const NOTICEABLE_SHARE = 0.15;

/** Действующие причины (вклад > 0) по убыванию силы; текст — с сервера, без него название признака. */
export function activeReasons(
  features: readonly CustomerFeature[],
  title: (id: string) => string,
): RiskReason[] {
  const active = byContribution(features).filter((f) => f.contribution > 0);
  const total = active.reduce((sum, f) => sum + f.contribution, 0);
  return active.map((f) => {
    const part = total > 0 ? f.contribution / total : 0;
    return {
      id: f.id,
      text: f.reason || title(f.id),
      strength: part >= STRONG_SHARE ? 'strong' : part >= NOTICEABLE_SHARE ? 'noticeable' : 'weak',
    };
  });
}

/** Услуги с потерей > 0 по убыванию потери: доля в потере клиента и причины каждой. */
export function riskReasons(
  risks: readonly ServiceRisk[],
  title: (id: string) => string,
): ServiceReasons[] {
  const lossy = risks.filter((r) => r.lossCents > 0).sort((a, b) => b.lossCents - a.lossCents);
  const total = lossy.reduce((sum, r) => sum + r.lossCents, 0);
  return lossy.map((r) => ({
    serviceId: r.serviceId,
    lossCents: r.lossCents,
    share: r.lossCents / total,
    reasons: activeReasons(r.features, title),
  }));
}
