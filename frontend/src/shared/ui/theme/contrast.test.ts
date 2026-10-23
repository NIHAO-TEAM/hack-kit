import { contrastPairs, ratio } from './contrast';
import { colorTokens, tokenNames } from './tokens';

describe.each(['light', 'dark'] as const)('контраст, тема %s', (mode) => {
  it.each(contrastPairs)('$fg на $bg не ниже $min', ({ fg, bg, min }) => {
    expect(ratio(colorTokens[mode][fg], colorTokens[mode][bg])).toBeGreaterThanOrEqual(min);
  });

  it('в обеих темах одни и те же токены', () => {
    expect(Object.keys(colorTokens[mode]).sort()).toEqual([...tokenNames].sort());
  });
});
