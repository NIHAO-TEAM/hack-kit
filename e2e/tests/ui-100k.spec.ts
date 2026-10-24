import { appendFileSync, writeFileSync } from 'node:fs';
import { expect, test, type APIRequestContext, type Locator, type Page } from '@playwright/test';

// Экраны на портфеле 100 000 клиентов (изменение ui-at-100k-check). Идёт только по одноразовому стенду
// с уже загруженным снимком (deploy-ui-100k.yml): предпросмотр веса и выгрузка меняют журнал, прод не трогаем.
// Цели [ПРЕДПОЛОЖЕНО] — для 4 vCPU раннера. Итог — таблица «экран — время — цель» в UI_100K_REPORT.
const juryPassword = process.env.E2E_JURY_PASSWORD ?? '';
const reportPath = process.env.UI_100K_REPORT ?? 'ui-100k-report.md';

const SCREEN_MS = 2000;
const SORT_MS = 1000;
const PREVIEW_MS = 2000;
const CSV_MEM_MB = 100;
const CSV_ROWS = 100_000;

const rows: { what: string; ms: number; goal: number }[] = [];
const bad: string[] = [];
const record = (what: string, ms: number, goal: number) => {
  rows.push({ what, ms: Math.round(ms), goal });
  if (ms > goal) bad.push(`${what}: ${Math.round(ms)} мс при цели ${goal} мс`);
};

const heading = (name?: string) => (p: Page) => p.getByRole('heading', name ? { name } : { level: 1 });
const queueTable = (p: Page) => p.getByRole('table', { name: 'Клиенты под риском оттока' });

const screens: { name: string; path: string; ready: (p: Page) => Locator }[] = [
  { name: 'очередь', path: '/queue', ready: queueTable },
  { name: 'сводка', path: '/summary', ready: heading('Где теряем деньги') },
  { name: 'задачи', path: '/tasks', ready: heading() },
  { name: 'модель', path: '/model', ready: heading() },
  { name: 'источник', path: '/source', ready: heading() },
  { name: 'о модели (устойчивость очереди)', path: '/about', ready: heading('Насколько устойчива очередь') },
];

async function metric(request: APIRequestContext, name: string): Promise<number> {
  const res = await request.get('/metrics');
  expect(res.ok(), '/metrics').toBeTruthy();
  const m = (await res.text()).match(new RegExp(`^${name}\\s+([0-9.eE+]+)$`, 'm'));
  expect(m, `метрика ${name}`).toBeTruthy();
  return Number(m![1]);
}

async function timed(fn: () => Promise<unknown>): Promise<number> {
  const t = Date.now();
  await fn();
  return Date.now() - t;
}

test.describe.configure({ mode: 'serial' });

test.beforeAll(() => {
  test.skip(!juryPassword, 'нужен E2E_JURY_PASSWORD');
});

test.afterAll(() => {
  const lines = ['| Экран | Время, мс | Цель, мс | Итог |', '|---|---|---|---|'];
  for (const r of rows) lines.push(`| ${r.what} | ${r.ms} | ${r.goal} | ${r.ms <= r.goal ? 'ок' : 'мимо'} |`);
  writeFileSync(reportPath, `${lines.join('\n')}\n`);
  if (process.env.GITHUB_STEP_SUMMARY) appendFileSync(process.env.GITHUB_STEP_SUMMARY, `## Экраны на 100k\n\n${lines.join('\n')}\n`);
});

test('экраны на 100 тыс. клиентов: время, консоль, пустые места, CSV', async ({ browser, baseURL }) => {
  test.setTimeout(600_000);
  const context = await browser.newContext({ baseURL });
  try {
    const login = await context.request.post('/api/v1/auth/login', { data: { login: 'jury', password: juryPassword } });
    expect(login.ok(), 'вход jury').toBeTruthy();
    const page = await context.newPage();

    const consoleErrors: string[] = [];
    page.on('console', (m) => m.type() === 'error' && consoleErrors.push(`${page.url()}: ${m.text()}`));
    page.on('pageerror', (e) => consoleErrors.push(`${page.url()}: ${e.message}`));
    const failedApi: string[] = [];
    page.on('response', (r) => r.url().includes('/api/') && r.status() >= 500 && failedApi.push(`${r.status()} ${r.url()}`));

    // Готовность снимка: очередь отдаёт 100 000 строк.
    const total = ((await (await context.request.get('/api/v1/customers?limit=1')).json()) as { total: number }).total;
    expect(total, 'в стенде загружен снимок 100k').toBeGreaterThanOrEqual(CSV_ROWS);

    // 1.1. Каждый экран: время до главного элемента, пустые места.
    for (const s of screens) {
      const ms = await timed(async () => {
        await page.goto(s.path);
        await expect(s.ready(page)).toBeVisible({ timeout: 30_000 });
      });
      record(`экран: ${s.name}`, ms, SCREEN_MS);
      await expect(page.getByRole('alert'), `${s.name}: сообщение об ошибке`).toHaveCount(0);
    }
    const cardMs = await timed(async () => {
      await page.goto('/queue/9');
      await expect(page.getByRole('heading', { name: 'Что сделать' })).toBeVisible({ timeout: 30_000 });
    });
    record('экран: карточка клиента', cardMs, SCREEN_MS);

    // Очередь: строки на месте, не пустая и не обрезана.
    await page.goto('/queue');
    await expect(queueTable(page)).toBeVisible();
    expect(await queueTable(page).getByRole('row').count(), 'строки очереди').toBeGreaterThan(10);

    // 1.2. Сортировка и страницы очереди (клик по UI).
    const header = queueTable(page).getByRole('columnheader').getByRole('button').first();
    const sortMs = await timed(async () => {
      await header.click();
      await expect(queueTable(page).getByRole('row').nth(1)).toBeVisible();
      await page.waitForLoadState('networkidle');
    });
    record('очередь: смена сортировки', sortMs, SORT_MS);
    const next = page.getByRole('button', { name: /Следующая|Вперёд/i }).first();
    if (await next.isVisible().catch(() => false)) {
      record('очередь: следующая страница', await timed(async () => {
        await next.click();
        await page.waitForLoadState('networkidle');
      }), SORT_MS);
    }
    const apiPage = await timed(async () => {
      const res = await context.request.get('/api/v1/customers?sort=score&order=asc&limit=50&offset=99900');
      expect(res.ok()).toBeTruthy();
    });
    record('API: последняя страница очереди', apiPage, SORT_MS);

    // Предпросмотр веса и устойчивость.
    const csrf = (await (await context.request.get('/api/v1/auth/me')).json()) as { csrfToken?: string };
    const preview = await timed(async () => {
      const res = await context.request.patch('/api/v1/model/settings?dryRun=true', {
        data: { weights: { noauto: 0.5 } },
        headers: { 'X-CSRF-Token': csrf.csrfToken ?? '' },
      });
      expect(res.ok(), 'предпросмотр веса').toBeTruthy();
    });
    record('API: предпросмотр веса (dryRun)', preview, PREVIEW_MS);
    const sens = await timed(async () => {
      const res = await context.request.get('/api/v1/model/sensitivity');
      expect(res.ok(), 'устойчивость').toBeTruthy();
    });
    record('API: устойчивость очереди', sens, PREVIEW_MS);

    // 1.3. CSV целиком, память процесса.
    const before = await metric(context.request, 'process_resident_memory_bytes');
    let csv = '';
    const csvMs = await timed(async () => {
      const res = await context.request.get('/api/v1/customers/export.csv');
      expect(res.ok(), 'CSV').toBeTruthy();
      csv = await res.text();
    });
    const after = await metric(context.request, 'process_resident_memory_bytes');
    const lines = csv.split('\n').filter((l) => l.trim() !== '').length - 1;
    rows.push({ what: `CSV: строк ${lines} из ${total}, ${Math.round(csvMs)} мс`, ms: Math.round(csvMs), goal: 0 });
    expect(lines, 'CSV отдан целиком').toBe(total);
    const growthMb = (after - before) / 1024 / 1024;
    rows.push({ what: `CSV: рост памяти процесса, МБ`, ms: Math.round(growthMb), goal: CSV_MEM_MB });
    if (growthMb > CSV_MEM_MB) bad.push(`CSV: память выросла на ${Math.round(growthMb)} МБ при цели ${CSV_MEM_MB} МБ`);

    expect(consoleErrors, 'ошибки консоли').toEqual([]);
    expect(failedApi, 'ответы 5xx').toEqual([]);
    expect(bad, 'цели не достигнуты').toEqual([]);
  } finally {
    await context.close();
  }
});
