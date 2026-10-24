import { linePoints, niceMax, plural, seriesOf, totalsLine, WINDOW_SECONDS } from './window';

describe('шкала и линия окна', () => {
  it('верх шкалы — круглое число не меньше максимума', () => {
    expect([0, 1, 3, 12, 480, 1200, 52000].map(niceMax)).toEqual([1, 1, 5, 20, 500, 2000, 100000]);
  });
  it('неполное окно прижато вправо: последняя точка на правом краю', () => {
    const pts = linePoints([0, 10], { width: 590, height: 100 }, 10);
    expect(pts[1]).toEqual({ x: 590, y: 0 });
    expect(pts[0]!.x).toBeCloseTo(580);
    expect(pts[0]!.y).toBe(100);
    expect(WINDOW_SECONDS).toBe(60);
  });
  it('шесть рядов, у «событий в секунду» две линии', () => {
    const s = seriesOf([
      {
        t: 1,
        accepted: 5,
        applied: 4,
        failed: 1,
        queueDepth: 7,
        batchSize: 2,
        lagP95Ms: 3,
        recalcs: 2,
      },
    ]);
    expect(s.map((x) => x.key)).toEqual(['rate', 'queue', 'batch', 'lag', 'recalcs', 'failed']);
    expect(s[0]!.lines.map((l) => l.values)).toEqual([[5], [4]]);
  });
});

describe('итоговая строка', () => {
  it('нули', () => {
    expect(totalsLine({ events: 10, applied: 10, failed: 0, consistency: { diffCents: 0 } })).toBe(
      'Ошибок 0 · потерянных 0 · согласованность 0 центов',
    );
  });
  it('потерянные и склонение', () => {
    expect(totalsLine({ events: 10, applied: 7, failed: 1, consistency: { diffCents: 1 } })).toBe(
      'Ошибок 1 · потерянных 2 · согласованность 1 цент',
    );
    expect(plural(3, ['цент', 'цента', 'центов'])).toBe('цента');
    expect(plural(11, ['цент', 'цента', 'центов'])).toBe('центов');
  });
});
