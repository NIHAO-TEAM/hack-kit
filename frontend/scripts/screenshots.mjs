// Скриншоты по матрице для frontend-design-review.
// Запуск: pnpm shots -- --base http://localhost:5173 --routes /points/demo/stock,/points/demo/orders
// Нужен @playwright/test (Firefox) и, для проверки доступности, @axe-core/playwright.
import { mkdir } from 'node:fs/promises';
import { join } from 'node:path';
import { firefox } from 'playwright';
import AxeBuilder from '@axe-core/playwright';

const arg = (name, fallback) => {
  const i = process.argv.indexOf(`--${name}`);
  return i > -1 ? process.argv[i + 1] : fallback;
};

const base = arg('base', 'http://localhost:5173');
const routes = arg('routes', '/').split(',');
const out = arg('out', 'shots');

// ширина, высота, касание, темы, состояния
const matrix = [
  { w: 1440, h: 900, touch: false, themes: ['light', 'dark'], states: ['data', 'empty', 'error'] },
  { w: 820, h: 1180, touch: true, themes: ['light', 'dark'], states: ['data'] },
  { w: 390, h: 844, touch: true, themes: ['light'], states: ['data'] },
];

const browser = await firefox.launch();
let violations = 0;

for (const route of routes) {
  const dir = join(out, route.replace(/[/$]/g, '_').replace(/^_/, '') || 'root');
  await mkdir(dir, { recursive: true });

  for (const m of matrix) {
    for (const theme of m.themes) {
      const context = await browser.newContext({
        viewport: { width: m.w, height: m.h },
        hasTouch: m.touch,
        colorScheme: theme,
      });
      const page = await context.newPage();

      for (const state of m.states) {
        const url = new URL(route, base);
        if (state !== 'data') url.searchParams.set('mock', state);
        await page.goto(url.href, { waitUntil: 'networkidle' });
        await page.screenshot({ path: join(dir, `${m.w}-${theme}-${state}.png`), fullPage: true });

        if (state === 'data') {
          const { violations: found } = await new AxeBuilder({ page }).analyze();
          const serious = found.filter((v) => v.impact === 'serious' || v.impact === 'critical');
          for (const v of serious) console.log(`axe ${route} ${m.w} ${theme}: ${v.id} — ${v.help}`);
          violations += serious.length;
        }
      }
      await context.close();
    }
  }
}

await browser.close();
console.log(`Скриншоты: ${out}/. Нарушений axe serious/critical: ${violations}.`);
process.exit(violations ? 1 : 0);
