import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { fontFamily } from './theme';

const css = readFileSync(resolve(process.cwd(), 'src/shared/ui/theme/global.css'), 'utf8');

describe('шрифты ADR-0003', () => {
  it('цифры идут отдельным шрифтом с табличными цифрами: Digits стоит первым в стеке', () => {
    expect(fontFamily.startsWith("'Digits', 'Exo 2', 'Inter'")).toBe(true);
    expect(css).toMatch(/font-family: Digits;[\s\S]*unicode-range: U\+0030-0039/);
    expect(css).toMatch(/font-variant-numeric: tabular-nums/);
  });

  it('Onest не подключён, Exo 2 и Inter — с кириллицей', () => {
    expect(css).not.toMatch(/onest/i);
    expect(css).toMatch(/exo-2\/cyrillic-400\.css/);
    expect(css).toMatch(/inter\/cyrillic-400\.css/);
  });
});
