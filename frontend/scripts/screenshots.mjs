// Скриншоты по матрице для frontend-design-review.
// Запуск: pnpm shots -- --base http://localhost:8080 --routes /queue,/queue/9,/summary
// Вход — переменные окружения E2E_LOGIN (по умолчанию jury) и E2E_JURY_PASSWORD; без пароля экраны за входом не откроются.
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
const routes = arg('routes', '/login,/queue,/queue/9,/summary,/model,/source,/about').split(',');
const login = process.env.E2E_LOGIN ?? 'jury';
const password = process.env.E2E_JURY_PASSWORD ?? '';
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
      // Тему читает theme-init.js из localStorage до загрузки приложения.
      await context.addInitScript(
        (t) => localStorage.setItem('mantine-color-scheme-value', t),
        theme,
      );
      if (password) {
        const res = await context.request.post(new URL('/api/v1/auth/login', base).href, {
          data: { login, password },
        });
        if (!res.ok()) throw new Error(`вход ${login}: ${res.status()}`);
      }
      const page = await context.newPage();

      for (const state of m.states) {
        const url = new URL(route, base);
        if (state !== 'data') url.searchParams.set('mock', state);
        await page.goto(url.href, { waitUntil: 'load' });
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
