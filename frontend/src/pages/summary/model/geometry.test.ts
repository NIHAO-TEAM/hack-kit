import { formatCompact } from '@shared/lib';
import { layout, niceMax, niceStep, path } from './geometry';

const box = { width: 100, height: 60, left: 10, right: 10, top: 10, bottom: 10 };

describe('геометрия графика оттока', () => {
  it('факт и прогноз делят текущую неделю, граница «сейчас» — последний факт', () => {
    const g = layout(
      [
        { weekStart: '2026-10-05', actual: 2, forecast: null },
        { weekStart: '2026-10-12', actual: 1, forecast: 1 },
        { weekStart: '2026-10-19', actual: null, forecast: 3 },
      ],
      box,
    );
    expect(g.actual).toHaveLength(2);
    expect(g.forecast).toHaveLength(2);
    expect(g.nowX).toBe(g.xOf(1));
    expect(g.yMax).toBe(4);
  });

  it('верх шкалы круглый', () => {
    expect(niceMax(0)).toBe(4);
    expect(niceMax(7)).toBe(8);
    expect(niceMax(13)).toBe(20);
    expect(niceMax(100000)).toBe(100000);
    expect(niceMax(98765)).toBe(100000);
  });

  it('шаг сетки — 1, 2, 2,5 или 5 × 10^k', () => {
    expect([0.5, 1, 1.75, 3.25, 7, 12345].map(niceStep)).toEqual([1, 1, 2, 5, 10, 20000]);
  });

  it('большие числа сокращаются, малые остаются как есть', () => {
    const nb = String.fromCharCode(160);
    expect(formatCompact(0)).toBe('0');
    expect(formatCompact(4)).toBe('4');
    expect(formatCompact(999)).toBe('999');
    expect(formatCompact(1000)).toBe(`1${nb}тыс.`);
    expect(formatCompact(25000)).toBe(`25${nb}тыс.`);
    expect(formatCompact(1200000)).toBe(`1,2${nb}млн`);
  });

  it('path пуст для пустой линии', () => {
    expect(path([])).toBe('');
    expect(
      path([
        { x: 1, y: 2 },
        { x: 3, y: 4 },
      ]),
    ).toBe('M1.0 2.0 L3.0 4.0');
  });
});
