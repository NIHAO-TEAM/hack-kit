// Проверка собранного CSS: токены и шрифт не потерялись при сборке.
// Запуск после vite build: node scripts/check-built-css.mjs dist
import assert from 'node:assert/strict';
import { readdir, readFile } from 'node:fs/promises';
import { join, resolve } from 'node:path';

const dir = resolve(process.argv[2] || 'dist');
const html = await readFile(join(dir, 'index.html'), 'utf8');
const sheets = [...html.matchAll(/<link\b[^>]*rel="stylesheet"[^>]*href="([^"]+)"/g)];
assert(sheets.length, 'В собранном index.html нет таблицы стилей');

// Токены обеих тем выдаёт cssVariablesResolver темы Mantine (JS), поэтому ищем и в бандле.
const assets = (await readdir(join(dir, 'assets'))).filter((f) => f.endsWith('.js'));
const js = (await Promise.all(assets.map((f) => readFile(join(dir, 'assets', f), 'utf8')))).join(
  '\n',
);

const css = (
  await Promise.all(sheets.map(([, href]) => readFile(join(dir, href.replace(/^\//, '')), 'utf8')))
).join('\n');

// Значения из ADR-0002: токены обеих тем и Onest.
const required = [
  [/\btext-strong["'`]?\s*:\s*["'`]#/, 'токен text-strong'],
  [/\bprimary["'`]?\s*:\s*["'`]#/, 'токен primary'],
  [/\bwarn-tint["'`]?\s*:\s*["'`]#/, 'токен warn-tint'],
  [/\[data-mantine-color-scheme=["']?dark/, 'значения тёмной темы'],
  [/font-family\s*:[^;]*Onest/, 'шрифт интерфейса'],
];

const built = `${css}\n${js}`;
for (const [re, what] of required) assert(re.test(built), `В сборке нет: ${what}`);
console.log('Собранный CSS: токены обеих тем и шрифт на месте.');
