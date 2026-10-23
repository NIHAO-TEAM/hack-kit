import { describe, expect, it } from 'vitest';
import { pollingNote } from './polling';

const base = { paused: false, intervalSeconds: 60, errorsTotal: 0 };

describe('pollingNote', () => {
  it('норма', () => {
    expect(pollingNote(base, '2026-10-24T12:40:00Z')).toEqual({
      text: 'Опрос каждые 60 с, сбоев: 0.',
      failing: false,
    });
  });

  it('сбой показывает время данных', () => {
    const n = pollingNote(
      { ...base, errorsTotal: 2, lastError: 'биллинг ответил 502' },
      '2026-10-24T12:40:00Z',
    );
    expect(n.failing).toBe(true);
    expect(n.text).toContain('данные от 12:40 UTC');
    expect(n.text).toContain('биллинг ответил 502');
  });

  it('пауза главнее сбоя', () => {
    const n = pollingNote({ ...base, paused: true, lastError: 'x' }, '2026-10-24T12:40:00Z');
    expect(n.failing).toBe(false);
    expect(n.text).toContain('на паузе');
  });
});
