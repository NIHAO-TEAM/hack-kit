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

  it('смена статуса задачи — по-русски', () => {
    expect(changeText({ field: 'status', from: 'in_work', to: 'contacted' }, 'taskChanged')).toBe(
      'Статус задачи: В работе → Связались',
    );
    expect(changeText({ field: 'status', from: 'in_work', to: 'contacted' })).toBe(
      'Статус услуги: in_work → contacted',
    );
  });

  it('итог, канал, ответственный и заметка без текста', () => {
    expect(changeText({ field: 'outcome', from: '', to: 'renewed' }, 'taskChanged')).toBe(
      'Итог: Продлил',
    );
    expect(changeText({ field: 'channel', from: '', to: 'phone' }, 'taskRecorded')).toBe(
      'Канал: Телефон',
    );
    expect(changeText({ field: 'assignee', from: 'ann', to: 'bob' }, 'taskChanged')).toBe(
      'Ответственный: ann → bob',
    );
    expect(changeText({ field: 'note', from: '', to: 'изменена' }, 'taskChanged')).toBe(
      'Заметка: изменена',
    );
  });

  it('срок — датой и временем, неизвестное значение не теряется', () => {
    const text = changeText(
      { field: 'dueAt', from: '2026-10-24T09:00:00Z', to: '2026-10-25T09:00:00Z' },
      'taskChanged',
    );
    expect(text).toMatch(
      /^Срок: \d{2}\.\d{2}\.\d{4}, \d{2}:\d{2} → \d{2}\.\d{2}\.\d{4}, \d{2}:\d{2}$/,
    );
    expect(changeText({ field: 'status', from: 'x', to: 'toString' }, 'taskChanged')).toBe(
      'Статус задачи: x → toString',
    );
    expect(changeText({ field: 'zzz', from: 'a', to: 'b' }, 'taskChanged')).toBe('zzz: a → b');
  });

  it('выгрузка: фильтры простыми словами', () => {
    expect(changeText({ field: 'level', from: '', to: 'high' }, 'customersExported')).toBe(
      'Уровень риска: Высокий риск',
    );
    expect(changeText({ field: 'billing', from: '', to: 'only' }, 'customersExported')).toBe(
      'Биллинг: только «к биллингу»',
    );
    expect(changeText({ field: 'q', from: '', to: 'задан' }, 'customersExported')).toBe(
      'Поиск: задан',
    );
  });
});
