import { outcomeLabel, statusLabel } from './labels';

describe('подписи задачи', () => {
  it('статусы и итоги', () => {
    expect(statusLabel.in_work).toBe('В работе');
    expect(statusLabel.contacted).toBe('Связались');
    expect(outcomeLabel.refused).toBe('Ушёл');
    expect(outcomeLabel.no_answer).toBe('Не удалось связаться');
  });
});
