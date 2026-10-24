/** Подписи признаков: что признак видит в данных, без долей исхода. Единый словарь для «Модели» и «О модели». Вес по умолчанию — как в backend (`risk`). */
export const featureInfo: Record<string, { title: string; hint: string; defaultWeight: number }> = {
  pwait: {
    title: 'Услуга ждёт оплаты',
    hint: 'Услуга ждёт оплаты продления',
    defaultWeight: 0.8,
  },
  noauto: {
    title: 'Автопродление выключено',
    hint: 'Автопродление выключено, продление зависит от клиента',
    defaultWeight: 0.55,
  },
  funds: {
    title: 'Не хватает средств на продление',
    hint: 'Баланса не хватит на ближайшие списания',
    defaultWeight: 0.7,
  },
  fail: {
    title: 'Были неудачные платежи',
    hint: 'В истории есть неудачные попытки оплаты',
    defaultWeight: 0.4,
  },
  susp: {
    title: 'Услуга приостановлена',
    hint: 'Услуга приостановлена, но продолжает оплачиваться',
    defaultWeight: 0.35,
  },
  flight: {
    title: 'Реферальный баланс к выводу',
    hint: 'Доступно к выводу X из Y. Факт вывода в данных не виден; по умолчанию не влияет на риск',
    defaultWeight: 0,
  },
  bonus_locked: {
    title: 'Бонусы заблокированы',
    hint: 'Бонусы на счёте нельзя потратить',
    defaultWeight: 0,
  },
};

export function featureTitle(id: string): string {
  return featureInfo[id]?.title ?? id;
}

/** Подсказка для настроек: что видно в данных и что вес — экспертная оценка. */
export function featureHint(id: string): string {
  return `${featureInfo[id]?.hint ?? 'Влияет на оценку риска оттока'}. Вес — экспертная оценка, не измерение`;
}

/** Настройки по умолчанию для кнопки «Сбросить»: как `risk.Defaults()` в backend. */
export const defaultSettings = {
  weights: Object.fromEntries(Object.entries(featureInfo).map(([id, f]) => [id, f.defaultWeight])),
  thresholds: { high: 70, medium: 50 },
  horizonDays: 30,
  priceBasis: 'per_period' as const,
};
