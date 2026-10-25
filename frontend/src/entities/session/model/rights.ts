import type { components } from '@shared/api';

export type Role = components['schemas']['Role'];

export type Right =
  | 'queue.view'
  | 'summary.view'
  | 'customer.view'
  | 'tasks.view'
  | 'tasks.edit'
  | 'model.view'
  | 'model.edit'
  | 'source.view';

const user: readonly Right[] = [
  'queue.view',
  'summary.view',
  'customer.view',
  'tasks.view',
  'tasks.edit',
  'model.view',
  'model.edit',
  'source.view',
];

/** Права для вида. Настоящую проверку делает сервер для каждой операции. */
const matrix: Record<Role, readonly Right[]> = {
  marketer: user,
  analyst: user,
  admin: user,
  jury: user,
};

export function can(role: Role | undefined, right: Right): boolean {
  return role !== undefined && matrix[role].includes(right);
}

/**
 * Стартовая страница роли (ADR-0001, дополнение 25.10: маркетолог и аналитик — один пользователь).
 * Пользователь и жюри начинают с очереди: демо-сценарий идёт от входа к очереди.
 */
export function homePath(role: Role): '/queue' | '/model' {
  return role === 'admin' ? '/model' : '/queue';
}
