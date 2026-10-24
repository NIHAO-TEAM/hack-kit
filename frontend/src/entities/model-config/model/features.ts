/** Подписи признаков: смысл простыми словами. Вес по умолчанию — как в backend (`risk`). */
export const featureInfo: Record<string, { title: string; hint: string; defaultWeight: number }> = {
  pwait: {
    title: 'Услуга ждёт оплаты',
    hint: '80 % услуг в ожидании оплаты не продлят',
    defaultWeight: 0.8,
  },
  noauto: {
    title: 'Автопродление выключено',
    hint: 'Без автопродления клиент уходит, если не вернётся в срок',
    defaultWeight: 0.55,
  },
  funds: {
    title: 'Не хватает средств на продление',
    hint: 'Баланса не хватит на ближайшие списания',
    defaultWeight: 0.7,
  },
  fail: {
    title: 'Были неудачные платежи',
    hint: 'Клиент уже пробовал оплатить и не смог',
    defaultWeight: 0.4,
  },
  susp: {
    title: 'Услуга приостановлена',
    hint: 'Приостановленная услуга часто не возвращается',
    defaultWeight: 0.35,
  },
  flight: {
    title: 'Реферальный баланс к выводу',
    hint: 'Доступно к выводу X из Y. Факт вывода в данных не виден; по умолчанию не влияет на риск',
    defaultWeight: 0,
  },
  bonus_locked: {
    title: 'Бонусы заблокированы',
    hint: 'Бонусы на счёте нельзя потратить — клиент теряет повод остаться',
    defaultWeight: 0,
  },
};

export function featureTitle(id: string): string {
  return featureInfo[id]?.title ?? id;
}

export function featureHint(id: string): string {
  return featureInfo[id]?.hint ?? 'Влияет на оценку риска оттока';
}

/** Настройки по умолчанию для кнопки «Сбросить»: как `risk.Defaults()` в backend. */
export const defaultSettings = {
  weights: Object.fromEntries(Object.entries(featureInfo).map(([id, f]) => [id, f.defaultWeight])),
  thresholds: { high: 70, medium: 50 },
  horizonDays: 30,
  priceBasis: 'per_period' as const,
};
