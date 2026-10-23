import { changeText, clampPage, fieldLabel } from './labels';

describe('журнал: подписи', () => {
  it('вес признака — по названию, а не по ключу', () => {
    expect(changeText({ field: 'weights.noauto', from: '0.55', to: '0.6' })).toBe(
      'Вес: Автопродление выключено: 0.55 → 0.6',
    );
  });

  it('флаг автопродления — словами', () => {
    expect(changeText({ field: 'autoProlong', from: 'false', to: 'true' })).toBe(
      'Автопродление: выключено → включено',
    );
  });

  it('неизвестное поле не теряется', () => {
    expect(fieldLabel('weights.zzz')).toBe('Вес: zzz');
    expect(fieldLabel('new.field')).toBe('new.field');
  });

  it('страница не выходит за границы', () => {
    expect(clampPage(5, 25, 10)).toBe(2);
    expect(clampPage(-1, 25, 10)).toBe(0);
    expect(clampPage(3, 0, 10)).toBe(0);
  });
});
