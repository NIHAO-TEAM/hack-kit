import type { Right, Role } from './rights';

export const roleLabel: Record<Role, string> = {
  marketer: 'маркетолог',
  analyst: 'аналитик',
  admin: 'администратор',
  jury: 'жюри',
};

/** У кого есть раздел — для объяснения на экране «Недостаточно прав». */
export const rightHolders: Record<Right, string> = {
  'queue.view': 'всем ролям',
  'summary.view': 'всем ролям',
  'customer.view': 'всем ролям',
  'tasks.view': 'всем ролям',
  'tasks.edit': 'маркетологу, администратору и жюри',
  'model.view': 'аналитику, администратору и жюри',
  'model.edit': 'аналитику, администратору и жюри',
  'source.view': 'аналитику, администратору и жюри',
};
