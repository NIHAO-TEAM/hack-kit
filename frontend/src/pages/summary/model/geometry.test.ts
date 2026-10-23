import { layout, niceMax, path } from './geometry';

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
    expect(niceMax(13)).toBe(16);
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
