// Проверка собранного CSS: токены и шрифт не потерялись при сборке (идея RIMS, UMIRHack 2026).
// Запуск после vite build: node scripts/check-built-css.mjs dist
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { join, resolve } from 'node:path';

const dir = resolve(process.argv[2] || 'dist');
const html = await readFile(join(dir, 'index.html'), 'utf8');
const sheets = [...html.matchAll(/<link\b[^>]*rel="stylesheet"[^>]*href="([^"]+)"/g)];
assert(sheets.length, 'В собранном index.html нет таблицы стилей');

const css = (
  await Promise.all(sheets.map(([, href]) => readFile(join(dir, href.replace(/^\//, '')), 'utf8')))
).join('\n');

// Заполнить в день 0 значениями из ADR-0002.
const required = [
  [/--text\s*:/, 'токен --text'],
  [/--primary\s*:/, 'токен --primary'],
  [/--warn-tint\s*:/, 'токен --warn-tint'],
  [/\[data-mantine-color-scheme=["']?dark/, 'значения тёмной темы'],
  [/font-family\s*:[^;]*<Шрифт из ADR-0002>/, 'шрифт интерфейса'],
];

for (const [re, what] of required) assert(re.test(css), `В собранном CSS нет: ${what}`);
console.log('Собранный CSS: токены обеих тем и шрифт на месте.');
