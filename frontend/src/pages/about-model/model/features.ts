/** Признаки модели для экрана «О модели». Правила — docs/model-rules.md, числа — оракул. */
export type FeatureInfo = {
  id: string;
  title: string;
  meaning: string;
  defaultWeight: number;
};

export const features: readonly FeatureInfo[] = [
  {
    id: 'pwait',
    title: 'Ожидание продления',
    meaning: 'Продление не прошло, до удаления услуги остаётся не больше 5 дней.',
    defaultWeight: 0.8,
  },
  {
    id: 'funds',
    title: 'Не хватает средств',
    meaning: 'На аккаунте меньше денег, чем нужно на ближайшие 7 дней.',
    defaultWeight: 0.7,
  },
  {
    id: 'noauto',
    title: 'Автопродление выключено',
    meaning: 'Срок близко, а продление зависит от клиента.',
    defaultWeight: 0.55,
  },
  {
    id: 'fail',
    title: 'Сбои продлений',
    meaning: 'Доля неудачных продлений услуги в истории.',
    defaultWeight: 0.4,
  },
  {
    id: 'susp',
    title: 'Услуга выключена',
    meaning: 'Клиент выключил услугу, но она продолжает оплачиваться.',
    defaultWeight: 0.35,
  },
  {
    id: 'flight',
    title: 'Вывод средств',
    meaning: 'Доля выведенных средств от баланса. Слабый, косвенный сигнал.',
    defaultWeight: 0.15,
  },
];

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
