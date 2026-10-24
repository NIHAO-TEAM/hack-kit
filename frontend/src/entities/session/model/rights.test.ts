import { can, homePath, type Right, type Role } from './rights';

const all: Right[] = [
  'queue.view',
  'summary.view',
  'customer.view',
  'tasks.view',
  'model.view',
  'model.edit',
  'source.view',
];

describe('can', () => {
  it('маркетолог видит очередь, сводку и карточку, но не модель и источник', () => {
    expect(all.filter((r) => can('marketer', r))).toEqual([
      'queue.view',
      'summary.view',
      'customer.view',
      'tasks.view',
    ]);
  });

  it.each<Role>(['analyst', 'admin', 'jury'])('%s видит все разделы', (role) => {
    expect(all.every((r) => can(role, r))).toBe(true);
  });

  it('без роли прав нет', () => {
    expect(all.some((r) => can(undefined, r))).toBe(false);
  });
});

describe('can: tasks.edit', () => {
  it.each<Role>(['marketer', 'admin', 'jury'])('%s работает с задачами', (role) => {
    expect(can(role, 'tasks.edit')).toBe(true);
  });

  it('аналитик задачи только смотрит', () => {
    expect(can('analyst', 'tasks.edit')).toBe(false);
  });
});

describe('homePath', () => {
  it('маркетолог и жюри начинают с очереди, аналитик и администратор с модели', () => {
    expect(homePath('marketer')).toBe('/queue');
    expect(homePath('jury')).toBe('/queue');
    expect(homePath('analyst')).toBe('/model');
    expect(homePath('admin')).toBe('/model');
  });
});
