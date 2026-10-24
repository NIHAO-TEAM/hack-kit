import { dueText, fromLocalInput, outcomeLabel, statusLabel, toLocalInput } from './labels';

const NOW = Date.parse('2026-10-24T12:00:00Z');

describe('подписи задачи', () => {
  it('статусы и итоги', () => {
    expect(statusLabel.in_work).toBe('В работе');
    expect(statusLabel.contacted).toBe('Связались');
    expect(outcomeLabel.no_answer).toBe('Не ответил');
  });

  it('просрочено: «срок прошёл 3 ч назад»', () => {
    expect(dueText('2026-10-24T09:00:00Z', NOW)).toEqual({
      text: 'срок прошёл 3 ч назад',
      overdue: true,
    });
  });

  it('не просрочено: сколько осталось', () => {
    expect(dueText('2026-10-24T12:30:00Z', NOW)).toEqual({ text: 'осталось 30 мин', overdue: false });
    expect(dueText('2026-10-30T12:00:00Z', NOW).text).toBe('осталось 6 д');
  });

  it('поле datetime-local туда и обратно', () => {
    const iso = '2026-10-25T15:00:00.000Z';
    expect(fromLocalInput(toLocalInput(iso))).toBe(iso);
    expect(fromLocalInput('')).toBeUndefined();
  });
});
