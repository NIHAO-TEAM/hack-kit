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

/** Строки блока: подписи из словаря признаков, порядок как в ответе сервера (по убыванию потери). */
export function reasonRows(items: readonly ReasonLoss[]): ReasonRow[] {
  return items.map((i) => ({
    ...i,
    title: featureTitle(i.featureId),
    share: formatShare(i.sharePercent),
    width: Math.max(0, Math.min(100, i.sharePercent)),
  }));
}
