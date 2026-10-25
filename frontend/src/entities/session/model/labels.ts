import type { Right, Role } from './rights';

export const roleLabel: Record<Role, string> = {
  marketer: 'пользователь',
  analyst: 'пользователь',
  admin: 'администратор',
  jury: 'жюри',
  viewer: 'гость',
};

/** У кого есть раздел — для объяснения на экране «Недостаточно прав». */
export const rightHolders: Record<Right, string> = {
  'queue.view': 'всем ролям',
  'summary.view': 'всем ролям',
  'customer.view': 'всем ролям',
  'tasks.view': 'всем ролям',
  'tasks.edit': 'пользователю, администратору и жюри',
  'model.view': 'пользователю, администратору и жюри',
  'model.edit': 'пользователю, администратору и жюри',
  'source.view': 'пользователю, администратору и жюри',
  'audit.view': 'пользователю, администратору и жюри',
  'sensitivity.view': 'пользователю, администратору и жюри',
};
