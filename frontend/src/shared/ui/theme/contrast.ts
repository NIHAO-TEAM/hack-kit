import type { TokenName } from './tokens';

const luminance = (hex: string) => {
  const [r, g, b] = [1, 3, 5].map((i) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255;
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r! + 0.7152 * g! + 0.0722 * b!;
};

export const ratio = (a: string, b: string) => {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi! + 0.05) / (lo! + 0.05);
};

/** Каждая пара «текст на фоне», которая встречается в интерфейсе. */
export const contrastPairs: ReadonlyArray<{ fg: TokenName; bg: TokenName; min: number }> = [
  ...(['bg', 'surface', 'surface-sunken'] as const).flatMap((bg) => [
    { fg: 'text-strong' as const, bg, min: 4.5 },
    { fg: 'text' as const, bg, min: 4.5 },
    { fg: 'text-muted' as const, bg, min: 4.5 },
    { fg: 'text-subtle' as const, bg, min: 4.5 },
    { fg: 'primary-fg' as const, bg, min: 4.5 },
    { fg: 'warn-fg' as const, bg, min: 4.5 },
    { fg: 'critical-fg' as const, bg, min: 4.5 },
    { fg: 'effect-fg' as const, bg, min: 4.5 },
    { fg: 'lost-fg' as const, bg, min: 4.5 },
  ]),
  { fg: 'on-primary', bg: 'primary', min: 4.5 },
  { fg: 'on-primary', bg: 'primary-hover', min: 4.5 },
  { fg: 'warn-on-tint', bg: 'warn-tint', min: 4.5 },
  { fg: 'critical-on-tint', bg: 'critical-tint', min: 4.5 },
  { fg: 'effect-on-tint', bg: 'effect-tint', min: 4.5 },
  { fg: 'lost-on-tint', bg: 'lost-tint', min: 4.5 },
  { fg: 'field-border', bg: 'surface', min: 3 },
  { fg: 'focus-ring', bg: 'surface', min: 3 },
  { fg: 'warn-solid', bg: 'surface', min: 3 },
  { fg: 'critical-solid', bg: 'surface', min: 3 },
  { fg: 'effect-solid', bg: 'surface', min: 3 },
  { fg: 'lost-solid', bg: 'surface', min: 3 },
];
