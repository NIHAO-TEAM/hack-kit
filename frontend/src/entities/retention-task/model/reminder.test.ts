import { endOfMskDay, dueSoon } from './reminder';
import type { RetentionTask } from '../api/task';

const task = (dueAt: string, status: RetentionTask['status'] = 'in_work') =>
  ({ dueAt, status }) as RetentionTask;

describe('напоминание', () => {
  it('конец дня — полночь по Москве', () => {
    expect(new Date(endOfMskDay(Date.parse('2026-10-24T10:00:00Z'))).toISOString()).toBe(
      '2026-10-24T21:00:00.000Z',
    );
    // 22:00 UTC — уже 25 октября по Москве
    expect(new Date(endOfMskDay(Date.parse('2026-10-24T22:00:00Z'))).toISOString()).toBe(
      '2026-10-25T21:00:00.000Z',
    );
  });

  it('берёт просроченные и сегодняшние открытые, пропускает закрытые и завтрашние', () => {
    const now = Date.parse('2026-10-24T10:00:00Z');
    const got = dueSoon(
      [
        task('2026-10-24T08:00:00Z'),
        task('2026-10-24T20:00:00Z'),
        task('2026-10-24T21:30:00Z'),
        task('2026-10-24T08:00:00Z', 'closed'),
      ],
      now,
    );
    expect(got.map((t) => t.dueAt)).toEqual(['2026-10-24T08:00:00Z', '2026-10-24T20:00:00Z']);
  });
});
