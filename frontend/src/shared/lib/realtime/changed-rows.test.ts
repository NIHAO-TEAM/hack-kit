import { act, renderHook } from '@testing-library/react';
import { HIGHLIGHT_MS, useChangedRows, type RowSignature } from './changed-rows';

const rows = (...s: string[]): RowSignature[] =>
  s.map((signature, i) => ({ id: i + 1, signature }));

describe('useChangedRows', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('первая загрузка не подсвечивается', () => {
    const { result } = renderHook(() => useChangedRows(rows('a', 'b'), 1));
    expect(result.current.size).toBe(0);
  });

  it('после нового пересчёта подсвечивает изменившиеся строки на 2 секунды', () => {
    const { result, rerender } = renderHook(({ items, v }) => useChangedRows(items, v), {
      initialProps: { items: rows('a', 'b'), v: 1 },
    });
    rerender({ items: rows('a', 'c'), v: 2 });
    expect([...result.current]).toEqual([2]);
    act(() => {
      vi.advanceTimersByTime(HIGHLIGHT_MS);
    });
    expect(result.current.size).toBe(0);
  });

  it('обновление без нового пересчёта не подсвечивает', () => {
    const { result, rerender } = renderHook(({ items, v }) => useChangedRows(items, v), {
      initialProps: { items: rows('a'), v: 1 },
    });
    rerender({ items: rows('z'), v: 1 });
    expect(result.current.size).toBe(0);
  });
});
