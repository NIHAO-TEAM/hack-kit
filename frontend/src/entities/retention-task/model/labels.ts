import type { components } from '@shared/api';

type Status = components['schemas']['TaskStatus'];
type Outcome = components['schemas']['TaskOutcome'];
type Channel = components['schemas']['TaskChannel'];

export const statusLabel: Record<Status, string> = {
  in_work: 'В работе',
  contacted: 'Связались',
  postponed: 'Отложено',
  closed: 'Закрыто',
};

export const outcomeLabel: Record<Outcome, string> = {
  renewed: 'Продлил',
  refused: 'Отказался',
  no_answer: 'Не ответил',
  not_needed: 'Не нужно',
};

export const channelLabel: Record<Channel, string> = {
  mail: 'Почта',
  phone: 'Телефон',
  messenger: 'Мессенджер',
  other: 'Другое',
};

const MIN = 60_000;

function span(ms: number): string {
  const min = Math.max(1, Math.floor(ms / MIN));
  if (min < 60) return `${min} мин`;
  const hours = Math.floor(min / 60);
  if (hours < 48) return `${hours} ч`;
  return `${Math.floor(hours / 24)} д`;
}

/** Срок словами: «срок прошёл 3 ч назад» или «осталось 2 ч». Точное время — в подсказке. */
export function dueText(dueAt: string, now: number): { text: string; overdue: boolean } {
  const diff = new Date(dueAt).getTime() - now;
  return diff < 0
    ? { text: `срок прошёл ${span(-diff)} назад`, overdue: true }
    : { text: `осталось ${span(diff)}`, overdue: false };
}

/** Значение для поля `datetime-local` (местное время браузера) из ISO. */
export function toLocalInput(iso: string): string {
  const d = new Date(iso);
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`;
}

/** ISO из значения поля `datetime-local`; пустое или неразобранное — `undefined`. */
export function fromLocalInput(value: string): string | undefined {
  const d = new Date(value);
  return value && !Number.isNaN(d.getTime()) ? d.toISOString() : undefined;
}
