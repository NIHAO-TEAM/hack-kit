import { changedCount, dryRunText } from './dry-run';

describe('dryRunText', () => {
  it('один клиент, потеря растёт', () => {
    expect(dryRunText({ levelChanges: { 'medium->high': 1 }, expectedLossDeltaCents: 20069 })).toBe(
      'сменят уровень 1 клиент, потеря +200,69 €',
    );
  });

  it('склоняет и суммирует переходы', () => {
    const r = {
      levelChanges: { 'low->medium': 2, 'medium->high': 3 },
      expectedLossDeltaCents: -500,
    };
    expect(changedCount(r)).toBe(5);
    expect(dryRunText(r)).toBe('сменят уровень 5 клиентов, потеря −5,00 €');
  });
});
