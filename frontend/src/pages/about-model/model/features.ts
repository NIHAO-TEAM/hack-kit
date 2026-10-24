import { featureInfo } from '@entities/model-config';

/** Признаки модели для экрана «О модели». Правила — docs/model-rules.md, числа — оракул. */
export type FeatureInfo = {
  id: string;
  title: string;
  meaning: string;
  defaultWeight: number;
};

/** Тот же словарь, что на экране «Модель»: новый признак появляется здесь сам. */
export const features: readonly FeatureInfo[] = Object.entries(featureInfo).map(([id, f]) => ({
  id,
  title: f.title,
  meaning: f.hint,
  defaultWeight: f.defaultWeight,
}));

export const defaultThresholds = { high: 70, medium: 50 } as const;

/** Вес признака: из настроек сервера, а если там его нет — по умолчанию. */
export function weightOf(
  feature: FeatureInfo,
  weights: Record<string, number> | undefined,
): number {
  return weights?.[feature.id] ?? feature.defaultWeight;
}

/** Вес с запятой: «0,55». */
export function formatWeight(weight: number): string {
  return weight.toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}
