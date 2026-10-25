import { featureTitle } from '@entities/model-config';

const NBSP = String.fromCharCode(160);

export type ReasonLoss = {
  featureId: string;
  lossCents: number;
  customers: number;
  sharePercent: number;
};

export type ReasonRow = ReasonLoss & { title: string; share: string; width: number };

/** Доля с запятой и двумя знаками: «51,24 %». */
export function formatShare(percent: number): string {
  return `${percent.toFixed(2).replace('.', ',')}${NBSP}%`;
}

/** Ненулевая доля не уже видимого минимума: 0,11 % иначе рисуется полосой в пиксель. */
export const MIN_BAR_PERCENT = 2;

/** Ширина полосы в процентах: 0 остаётся 0, остальное — от минимума до 100. */
export function barWidth(percent: number): number {
  if (!(percent > 0)) return 0;
  return Math.min(100, Math.max(MIN_BAR_PERCENT, percent));
}

/** Строки блока: подписи из словаря признаков, порядок как в ответе сервера (по убыванию потери). */
export function reasonRows(items: readonly ReasonLoss[]): ReasonRow[] {
  return items.map((i) => ({
    ...i,
    title: featureTitle(i.featureId),
    share: formatShare(i.sharePercent),
    width: barWidth(i.sharePercent),
  }));
}
