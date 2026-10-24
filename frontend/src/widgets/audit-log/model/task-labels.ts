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
  refused: 'Ушёл',
  no_answer: 'Не удалось связаться',
  not_needed: 'Не нужно',
};

export const channelLabel: Record<Channel, string> = {
  mail: 'Почта',
  phone: 'Телефон',
  messenger: 'Мессенджер',
  other: 'Другое',
};
