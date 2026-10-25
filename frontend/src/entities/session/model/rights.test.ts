import { can, homePath, type Right, type Role } from './rights';

const all: Right[] = [
  'queue.view',
  'summary.view',
  'customer.view',
  'tasks.view',
  'model.view',
  'model.edit',
  'source.view',
  'audit.view',
  'sensitivity.view',
];

describe('can', () => {
  it.each<Role>(['marketer', 'analyst', 'admin', 'jury'])('%s видит все разделы', (role) => {
    expect(all.every((r) => can(role, r))).toBe(true);
  });

  it('без роли прав нет', () => {
    expect(all.some((r) => can(undefined, r))).toBe(false);
  });
});

describe('can: tasks.edit', () => {
  it.each<Role>(['marketer', 'analyst', 'admin', 'jury'])('%s работает с задачами', (role) => {
    expect(can(role, 'tasks.edit')).toBe(true);
  });
});

describe('homePath', () => {
  it('пользователь (маркетолог, аналитик) и жюри начинают с очереди, администратор с модели', () => {
    expect(homePath('marketer')).toBe('/queue');
    expect(homePath('analyst')).toBe('/queue');
    expect(homePath('jury')).toBe('/queue');
    expect(homePath('admin')).toBe('/model');
  });
});

describe('can: viewer (гость)', () => {
  it.each<Right>(['queue.view', 'summary.view', 'customer.view', 'tasks.view', 'model.view'])(
    'читает: %s',
    (right) => {
      expect(can('viewer', right)).toBe(true);
    },
  );

  it.each<Right>(['tasks.edit', 'model.edit', 'source.view', 'audit.view', 'sensitivity.view'])(
    'не получает: %s',
    (right) => {
      expect(can('viewer', right)).toBe(false);
    },
  );

  it('начинает с очереди', () => {
    expect(homePath('viewer')).toBe('/queue');
  });
});
