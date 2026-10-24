import { formatCount, formatDuration, formatMs, formatRate, progressPercent } from './progress';

describe('поток событий: форматы', () => {
  it('доля прогона', () => {
    expect(progressPercent({ applied: 0, events: 0 })).toBe(0);
    expect(progressPercent({ applied: 1250, events: 2500 })).toBe(50);
    expect(progressPercent({ applied: 3000, events: 2500 })).toBe(100);
  });

  it('числа, миллисекунды, длительность, скорость', () => {
    expect(formatCount(2500)).toBe('2 500');
    expect(formatMs(0.83)).toBe('0,83 мс');
    expect(formatMs(18.46)).toBe('18,5 мс');
    expect(formatMs(1240)).toBe('1 240 мс');
    expect(formatDuration(47750)).toBe('47,8 с');
    expect(formatDuration(65000)).toBe('1 мин 05 с');
    expect(formatRate(52.43)).toBe('52,4 /с');
  });
});
