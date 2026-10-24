import { channelLabel, outcomeLabel, statusLabel } from './task-labels';
import type { AuditChange } from '../api/audit';

const features: Record<string, string> = {
  pwait: 'Услуга ждёт оплаты',
  noauto: 'Автопродление выключено',
  funds: 'Не хватает средств на продление',
  fail: 'Были неудачные платежи',
  susp: 'Услуга приостановлена',
  flight: 'Реферальный баланс к выводу',
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

const taskFields: Record<string, string> = {
  status: 'Статус задачи',
  assignee: 'Ответственный',
  dueAt: 'Срок',
  note: 'Заметка',
  outcome: 'Итог',
  channel: 'Канал',
};

const exportFields: Record<string, string> = {
  level: 'Уровень риска',
  billing: 'Биллинг',
  q: 'Поиск',
};

const levels: Record<string, string> = {
  high: 'Высокий риск',
  medium: 'Средний риск',
  low: 'Низкий риск',
  unscored: 'Без оценки',
  lost: 'Потерян',
};

const billingFilter: Record<string, string> = {
  exclude: 'без «к биллингу»',
  only: 'только «к биллингу»',
};

const when = new Intl.DateTimeFormat('ru-RU', { dateStyle: 'short', timeStyle: 'short' });

const isTask = (action?: string) => action?.startsWith('task') === true;

/** Название поля простыми словами; неизвестное показываем как есть, чтобы ничего не терять. */
export function fieldLabel(field: string, action?: string): string {
  const task = isTask(action) ? taskFields[field] : undefined;
  if (task) return task;
  const exported = action === 'customersExported' ? exportFields[field] : undefined;
  if (exported) return exported;
  if (field.startsWith('weights.')) {
    const id = field.slice('weights.'.length);
    return `Вес: ${features[id] ?? id}`;
  }
  return fields[field] ?? field;
}

const booleans: Record<string, string> = { true: 'включено', false: 'выключено' };

function dict(map: Record<string, string>, v: string): string {
  return (Object.hasOwn(map, v) ? map[v] : undefined) ?? v;
}

function taskValue(field: string, v: string): string {
  switch (field) {
    case 'status':
      return dict(statusLabel, v);
    case 'outcome':
      return dict(outcomeLabel, v);
    case 'channel':
      return dict(channelLabel, v);
    case 'dueAt': {
      const d = new Date(v);
      return Number.isNaN(d.getTime()) ? v : when.format(d);
    }
    default:
      return v;
  }
}

/** Значение поля простыми словами для действия; незнакомое оставляем как есть. */
function valueText(c: AuditChange, v: string, action?: string): string {
  if (isTask(action)) return taskValue(c.field, v);
  if (action === 'customersExported') {
    if (c.field === 'level') return dict(levels, v);
    if (c.field === 'billing') return dict(billingFilter, v);
    return v;
  }
  return c.field === 'autoProlong' ? dict(booleans, v) : v;
}

/** «0.55 → 0.6» с понятными значениями; запись без прежнего значения — просто «поле: значение». */
export function changeText(c: AuditChange, action?: string): string {
  const label = fieldLabel(c.field, action);
  if (c.from === '' && (isTask(action) || action === 'customersExported'))
    return `${label}: ${valueText(c, c.to, action)}`;
  return `${label}: ${valueText(c, c.from, action)} → ${valueText(c, c.to, action)}`;
}

/** Номер страницы после смены: не выходит за [0, last]. */
export function clampPage(page: number, total: number, size: number): number {
  const last = Math.max(0, Math.ceil(total / size) - 1);
  return Math.min(Math.max(0, page), last);
}
