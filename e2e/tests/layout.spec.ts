import { expect, test, type Locator, type Page } from '@playwright/test';

// Вёрстка на телефоне и планшете (шкала 5.9): без горизонтальной прокрутки страницы,
// цели касания от 44 px на планшете, главное действие видно на телефоне.
// Только читает прод: «Отметить: взято в работу» не нажимается. Пароль — из окружения.
const juryPassword = process.env.E2E_JURY_PASSWORD ?? '';

const MIN_TARGET = 44;

type Screen = {
  name: string;
  path: string;
  ready: (p: Page) => Locator;
  anon?: boolean;
  /** Главное действие экрана на телефоне; нет такой кнопки — проверяется только меню. */
  action?: (p: Page) => Locator;
};

const heading = (p: Page) => p.getByRole('heading', { level: 1 });
const screens: Screen[] = [
  { name: 'вход', path: '/login', ready: (p) => p.getByLabel('Логин'), anon: true, action: (p) => p.getByRole('button', { name: 'Войти' }) },
  { name: 'очередь', path: '/queue', ready: (p) => p.getByRole('table', { name: 'Клиенты под риском оттока' }) },
  {
    name: 'карточка клиента 9',
    path: '/queue/9',
    ready: heading,
    action: (p) => p.getByRole('button', { name: /^(Отметить: взято в работу|Закрыть)$/ }),
  },
  { name: 'сводка', path: '/summary', ready: heading },
  { name: 'модель', path: '/model', ready: heading },
  { name: 'источник', path: '/source', ready: heading },
  { name: 'о модели', path: '/about', ready: heading },
];

// Цели касания на планшете: шапка, меню разделов и кнопки главных действий экранов.
// Ссылки внутри текста в перечень не входят — WCAG 2.5.8 для них делает исключение.
const touchTargets = (p: Page): { what: string; loc: Locator }[] => [
  { what: 'шапка', loc: p.locator('header').getByRole('button') },
  { what: 'шапка', loc: p.locator('header').getByRole('link') },
  { what: 'шапка: тема', loc: p.locator('header').getByRole('radio') },
  { what: 'меню', loc: p.getByRole('navigation', { name: 'Разделы' }).getByRole('link') },
  { what: 'сортировка', loc: p.getByRole('columnheader').getByRole('button') },
  { what: 'Отметить: взято в работу', loc: p.getByRole('button', { name: 'Отметить: взято в работу' }) },
  { what: 'Закрыть', loc: p.getByRole('button', { name: 'Закрыть', exact: true }) },
  { what: 'страницы', loc: p.getByRole('button', { name: /страниц|Назад|Вперёд|Следующая|Предыдущая/i }) },
  { what: 'кнопка формы', loc: p.locator('main').getByRole('button', { name: /^(Войти|Отложить|Закрыть задачу|Экспорт.*)$/ }) },
];

const widths = [
  { w: 390, h: 844 },
  { w: 820, h: 1180 },
];

test.describe.configure({ mode: 'serial' });

test.beforeAll(() => {
  test.skip(!juryPassword, 'нужен E2E_JURY_PASSWORD');
});

for (const { w, h } of widths) {
  test(`вёрстка: ${w}px, тёмная тема`, async ({ browser, baseURL }, testInfo) => {
    const open = async () => {
      const ctx = await browser.newContext({ baseURL, viewport: { width: w, height: h } });
      // Тема читается theme-init.js из localStorage до загрузки приложения.
      await ctx.addInitScript(() => localStorage.setItem('mantine-color-scheme-value', 'dark'));
      return ctx;
    };
    const context = await open();
    const failures: string[] = [];
    try {
      const login = await context.request.post('/api/v1/auth/login', {
        data: { login: 'jury', password: juryPassword },
      });
      expect(login.ok(), 'вход jury').toBeTruthy();
      const page = await context.newPage();

      for (const screen of screens) {
        // Экран входа проверяем без сессии: её cookie перенаправила бы на очередь.
        const ctx = screen.anon ? await open() : context;
        const p = screen.anon ? await ctx.newPage() : page;
        const bad: string[] = [];
        await p.goto(screen.path);
        await expect(screen.ready(p).first(), `экран «${screen.name}»`).toBeVisible();

        // Таблицам можно прокручиваться в своей обёртке, странице — нет.
        const { scroll, inner } = await p.evaluate(() => ({
          scroll: document.documentElement.scrollWidth,
          inner: window.innerWidth,
        }));
        if (scroll > inner + 1) bad.push(`горизонтальная прокрутка страницы: ${scroll} > ${inner}`);

        if (w < 768) {
          const burger = p.getByRole('button', { name: 'Меню' });
          const candidates = screen.anon ? [] : [burger, ...(await p.locator('header').getByRole('button').all())];
          if (screen.action) candidates.push(screen.action(p).first());
          for (const loc of candidates) {
            if (!(await loc.count())) continue;
            const box = await loc.boundingBox();
            if (!box || box.x < 0 || box.x + box.width > w + 1) {
              bad.push(`главное действие вне экрана: ${box ? `x ${Math.round(box.x)}…${Math.round(box.x + box.width)}` : 'не видно'}`);
            }
          }
        } else {
          let checked = 0;
          for (const { what, loc } of touchTargets(p)) {
            for (const el of await loc.all()) {
              if (!(await el.isVisible())) continue;
              const box = await el.boundingBox();
              // Заголовки таблицы на планшете спрятаны (строки стали карточками): их кнопки не мишень.
              if (!box || box.width <= 2 || box.height <= 2) continue;
              checked++;
              if (box.width < MIN_TARGET - 0.5 || box.height < MIN_TARGET - 0.5) {
                const label = (await el.innerText().catch(() => '')).trim() || (await el.getAttribute('aria-label')) || '—';
                bad.push(`${what} «${label.slice(0, 40)}»: ${Math.round(box.width)}×${Math.round(box.height)} px`);
              }
            }
          }
          // Ни одной цели не нашлось — селекторы устарели, зелёный прогон был бы ложным.
          if (!screen.anon && checked === 0) bad.push('не найдено ни одной цели касания');
        }

        for (const b of bad) failures.push(`${screen.name} · ${w}px · ${b}`);
        if (bad.length) {
          await p.screenshot({ path: testInfo.outputPath(`${screen.name.replace(/\s+/g, '-')}-${w}.png`), fullPage: true });
        }
        if (screen.anon) await ctx.close();
      }
    } finally {
      await context.close();
    }
    expect(failures, `нарушения вёрстки:\n${failures.join('\n')}`).toEqual([]);
  });
}
