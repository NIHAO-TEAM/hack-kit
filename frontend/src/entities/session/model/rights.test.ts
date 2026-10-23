import { can, homePath, type Right, type Role } from './rights';

const all: Right[] = [
  'queue.view',
  'summary.view',
  'customer.view',
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
    ]);
  });

  it.each<Role>(['analyst', 'admin', 'jury'])('%s видит все разделы', (role) => {
    expect(all.every((r) => can(role, r))).toBe(true);
  });

  it('без роли прав нет', () => {
    expect(all.some((r) => can(undefined, r))).toBe(false);
  });
});

describe('homePath', () => {
  it('маркетолог начинает с очереди, остальные с модели', () => {
    expect(homePath('marketer')).toBe('/queue');
    expect(homePath('analyst')).toBe('/model');
  });
});
