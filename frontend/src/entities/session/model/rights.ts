import type { components } from '@shared/api';

export type Role = components['schemas']['Role'];

export type Right =
  'queue.view' | 'summary.view' | 'customer.view' | 'model.view' | 'model.edit' | 'source.view';

const marketer: readonly Right[] = ['queue.view', 'summary.view', 'customer.view'];
const staff: readonly Right[] = [...marketer, 'model.view', 'model.edit', 'source.view'];

/** Права для вида. Настоящую проверку делает сервер для каждой операции. */
const matrix: Record<Role, readonly Right[]> = {
  marketer,
  analyst: staff,
  admin: staff,
  jury: staff,
};

export function can(role: Role | undefined, right: Right): boolean {
  return role !== undefined && matrix[role].includes(right);
}

/** Стартовая страница роли (ADR-0001: маркетолог — очередь, аналитик — модель). */
export function homePath(role: Role): '/queue' | '/model' {
  return role === 'marketer' ? '/queue' : '/model';
}
