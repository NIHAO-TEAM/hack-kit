import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';

// Доступность экранов jury (шкала 5.9): axe, WCAG 2 AA, нарушения serious и critical.
// Только читает прод, данных не меняет. Пароль — из окружения.
const juryPassword = process.env.E2E_JURY_PASSWORD ?? '';

const screens = [
  { name: 'вход', path: '/login', ready: (p: Page) => p.getByLabel('Логин'), anon: true },
  { name: 'очередь', path: '/queue', ready: (p: Page) => p.getByRole('table', { name: 'Клиенты под риском оттока' }) },
  { name: 'карточка клиента 9', path: '/queue/9', ready: (p: Page) => p.getByRole('heading', { level: 1 }) },
  { name: 'сводка', path: '/summary', ready: (p: Page) => p.getByRole('heading', { level: 1 }) },
  { name: 'модель', path: '/model', ready: (p: Page) => p.getByRole('heading', { level: 1 }) },
  { name: 'источник', path: '/source', ready: (p: Page) => p.getByRole('heading', { level: 1 }) },
  { name: 'о модели', path: '/about', ready: (p: Page) => p.getByRole('heading', { level: 1 }) },
];
const themes = ['dark', 'light'] as const;
const widths = [
  { w: 1440, h: 900 },
  { w: 390, h: 844 },
];

test.describe.configure({ mode: 'serial' });

test.beforeAll(() => {
  test.skip(!juryPassword, 'нужен E2E_JURY_PASSWORD');
});

for (const { w, h } of widths) {
  for (const theme of themes) {
    test(`axe: ${w}px, тема ${theme}`, async ({ browser, baseURL }, testInfo) => {
      const context = await browser.newContext({ baseURL, viewport: { width: w, height: h } });
      // Тема читается theme-init.js из localStorage до загрузки приложения.
      await context.addInitScript((t) => localStorage.setItem('mantine-color-scheme-value', t), theme);
      const failures: string[] = [];
      try {
        const login = await context.request.post('/api/v1/auth/login', {
          data: { login: 'jury', password: juryPassword },
        });
        expect(login.ok(), 'вход jury').toBeTruthy();
        const page = await context.newPage();

        for (const screen of screens) {
          // Экран входа проверяем без сессии: её cookie перенаправила бы на очередь.
          const ctx = screen.anon ? await browser.newContext({ baseURL, viewport: { width: w, height: h } }) : context;
          const p = screen.anon ? await ctx.newPage() : page;
          if (screen.anon) await ctx.addInitScript((t) => localStorage.setItem('mantine-color-scheme-value', t), theme);
          await p.goto(screen.path);
          await expect(screen.ready(p).first(), `экран «${screen.name}»`).toBeVisible();

          const { violations } = await new AxeBuilder({ page: p }).withTags(['wcag2a', 'wcag2aa']).analyze();
          const bad = violations.filter((v) => v.impact === 'serious' || v.impact === 'critical');
          for (const v of bad) {
            for (const node of v.nodes) {
              failures.push(`${screen.name} · ${theme} · ${w}px · ${v.id} · ${node.target.join(' ')}`);
            }
          }
          if (bad.length) {
            await p.screenshot({
              path: testInfo.outputPath(`${screen.name.replace(/\s+/g, '-')}-${theme}-${w}.png`),
              fullPage: true,
            });
          }
          if (screen.anon) await ctx.close();
        }
      } finally {
        await context.close();
      }
      expect(failures, `нарушения axe:\n${failures.join('\n')}`).toEqual([]);
    });
  }
}
