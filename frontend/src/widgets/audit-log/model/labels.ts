import type { AuditChange } from '../api/audit';

const features: Record<string, string> = {
  pwait: 'Услуга ждёт оплаты',
  noauto: 'Автопродление выключено',
  funds: 'Не хватает средств на продление',
  fail: 'Были неудачные платежи',
  susp: 'Услуга приостановлена',
  flight: 'Баланс уходит со счёта',
  bonus_locked: 'Бонусы заблокированы',
};

const fields: Record<string, string> = {
  'thresholds.high': 'Порог высокого риска',
  'thresholds.medium': 'Порог среднего риска',
  horizonDays: 'Горизонт, дней',
  priceBasis: 'База цены',
  asOfMode: 'Дата расчёта',
  status: 'Статус услуги',
  autoProlong: 'Автопродление',
  expiresAt: 'Срок услуги',
  balance: 'Баланс, центы',
  bonusBalance: 'Бонусный баланс, центы',
};

/** Название поля простыми словами; неизвестное показываем как есть, чтобы ничего не терять. */
export function fieldLabel(field: string): string {
  if (field.startsWith('weights.')) {
    const id = field.slice('weights.'.length);
    return `Вес: ${features[id] ?? id}`;
  }
  return fields[field] ?? field;
}

const booleans: Record<string, string> = { true: 'включено', false: 'выключено' };

/** «0.55 → 0.6» с понятными значениями флагов. */
export function changeText(c: AuditChange): string {
  const show = (v: string) => (c.field === 'autoProlong' ? (booleans[v] ?? v) : v);
  return `${fieldLabel(c.field)}: ${show(c.from)} → ${show(c.to)}`;
}

/** Номер страницы после смены: не выходит за [0, last]. */
export function clampPage(page: number, total: number, size: number): number {
  const last = Math.max(0, Math.ceil(total / size) - 1);
  return Math.min(Math.max(0, page), last);
}
