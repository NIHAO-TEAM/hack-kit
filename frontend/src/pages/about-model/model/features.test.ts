import { features, formatWeight, weightOf } from './features';

describe('weightOf', () => {
  const susp = features.find((f) => f.id === 'susp')!;

  it('берёт вес из настроек сервера', () => {
    expect(weightOf(susp, { susp: 0.5 })).toBe(0.5);
  });

  it('без веса в настройках берёт вес по умолчанию', () => {
    expect(weightOf(susp, {})).toBe(0.35);
    expect(weightOf(susp, undefined)).toBe(0.35);
  });
});

describe('formatWeight', () => {
  it('пишет вес с запятой и двумя знаками', () => {
    expect(formatWeight(0.55)).toBe('0,55');
    expect(formatWeight(0.8)).toBe('0,80');
  });
});
