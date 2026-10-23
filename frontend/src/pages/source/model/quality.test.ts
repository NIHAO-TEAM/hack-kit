import { eurosToCents } from './quality';

describe('eurosToCents', () => {
  it.each([
    ['529,84', 52984],
    ['529.84', 52984],
    ['1 000', 100000],
    ['0', 0],
  ])('%s → %d', (text, cents) => expect(eurosToCents(text)).toBe(cents));

  it.each(['', 'abc', '-5', '1,234'])('«%s» не число', (text) =>
    expect(eurosToCents(text)).toBeNull(),
  );
});
