import { formatCountdown, formatMoney } from './format';

const H = 3_600_000;

describe('formatMoney', () => {
  it('центы в евро', () => {
    expect(formatMoney(387973)).toBe('3 879,73 €');
    expect(formatMoney(5)).toBe('0,05 €');
    expect(formatMoney(100000000)).toBe('1 000 000,00 €');
  });
  it('минус — U+2212', () => {
    expect(formatMoney(-36529)).toBe('−365,29 €');
  });
});

describe('formatCountdown', () => {
  it('дни и часы', () => {
    expect(formatCountdown(55 * H)).toBe('2 д 07 ч');
  });
  it('меньше суток', () => {
    expect(formatCountdown(7 * H + 5 * 60_000)).toBe('7 ч 05 мин');
    expect(formatCountdown(20 * 60_000)).toBe('20 мин');
    expect(formatCountdown(10_000)).toBe('меньше минуты');
  });
  it('прошедший срок не уходит в минус', () => {
    expect(formatCountdown(-H)).toBe('меньше минуты');
  });
});
