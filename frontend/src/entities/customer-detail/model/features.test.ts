import { byContribution, formulaText } from './features';

const features = [
  { id: 'susp', value: 1, weight: 0.35, contribution: 0.35, reason: '' },
  { id: 'funds', value: 1, weight: 0.7, contribution: 0.7, reason: '' },
];

describe('признаки карточки', () => {
  it('сортируются по убыванию вклада и не меняют исходный массив', () => {
    expect(byContribution(features).map((f) => f.id)).toEqual(['funds', 'susp']);
    expect(features[0]?.id).toBe('susp');
  });

  it('формула показывает числа ответа', () => {
    expect(formulaText(features, 0.8)).toBe('P = 1 − (1 − 0,7·1) · (1 − 0,35·1) = 0,8');
  });
});
