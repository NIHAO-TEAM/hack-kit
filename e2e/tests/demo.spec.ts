import { expect, test, type APIRequestContext, type Locator, type Page } from '@playwright/test';

// Сценарий — docs/demo-script.md. Учётки и пароли только из окружения.
const marketerPassword = process.env.E2E_MARKETER_PASSWORD ?? '';
const analystPassword = process.env.E2E_ANALYST_PASSWORD ?? '';
const juryPassword = process.env.E2E_JURY_PASSWORD ?? '';

const NBSP = ' ';
const money = (euros: string) => `${euros}${NBSP}€`;

interface Row {
  id: number;
  level: string;
  score: number | null;
  expectedLossCents: number;
}

// Сессия по cookie: каждый запрос с изменением требует заголовок X-CSRF-Token (токен приходит при входе).
const csrfTokens = new WeakMap<APIRequestContext, string>();

async function apiLogin(request: APIRequestContext, login: string, password: string) {
  const res = await request.post('/api/v1/auth/login', { data: { login, password } });
  expect(res.ok(), `вход ${login}`).toBeTruthy();
  const { csrfToken } = (await res.json()) as { csrfToken?: string };
  expect(csrfToken, `CSRF-токен ${login}`).toBeTruthy();
  csrfTokens.set(request, csrfToken ?? '');
}

function mutate(request: APIRequestContext, method: 'post' | 'patch', url: string, data: unknown) {
  return request[method](url, { data, headers: { 'X-CSRF-Token': csrfTokens.get(request) ?? '' } });
}

async function uiLogin(page: Page, login: string, password: string) {
  await page.goto('/login');
  await page.getByLabel('Логин').fill(login);
  await page.getByLabel('Пароль').fill(password);
  await page.getByRole('button', { name: 'Войти' }).click();
  await expect(page.getByRole('table', { name: 'Клиенты под риском оттока' })).toBeVisible();
}

// Переход, после которого страница доведена до устойчивого состояния: следующий goto не обрывает загрузку (Firefox: NS_BINDING_ABORTED).
async function gotoStable(page: Page, path: string, ready: (page: Page) => Locator) {
  // 'domcontentloaded': страницы держат открытый поток событий (SSE), и 'load' в Firefox не гарантирован; готовность — по видимому элементу.
  await page.goto(path, { waitUntil: 'domcontentloaded' });
  await expect(ready(page)).toBeVisible();
}

const queueTable = (page: Page) => page.getByRole('table', { name: 'Клиенты под риском оттока' });

async function customers(request: APIRequestContext): Promise<Row[]> {
  const res = await request.get('/api/v1/customers?sort=expectedLoss&order=desc&limit=100');
  expect(res.ok()).toBeTruthy();
  return (await res.json()).items as Row[];
}

async function setNoauto(request: APIRequestContext, value: number) {
  const res = await mutate(request, 'patch', '/api/v1/model/settings', { weights: { noauto: value } });
  expect(res.ok(), 'правка веса noauto').toBeTruthy();
}

test.describe.configure({ mode: 'serial' });

test.beforeAll(() => {
  test.skip(!marketerPassword || !analystPassword, 'нужны E2E_MARKETER_PASSWORD и E2E_ANALYST_PASSWORD');
});

test('демо: очередь → карточка → потеря по причинам → вес → устойчивость → автопродление → поток событий → /metrics', async ({ browser, baseURL }) => {
  // Шаг потока 2 500 событий ждёт до 180 с, лимит по умолчанию (30 с) короче его.
  test.setTimeout(300_000);
  const marketerCtx = await browser.newContext({ baseURL });
  const analystCtx = await browser.newContext({ baseURL });
  const analyst = analystCtx.request;
  try {
    // Сброс стенда: данные эмулятора и вес noauto в начальное состояние.
    await apiLogin(analyst, 'analyst', analystPassword);
    const reset = await mutate(analyst, 'post', '/api/v1/source/emulator/actions', { actions: [{ customerId: 0, actionId: 'reset' }] });
    expect(reset.ok(), 'reset эмулятора').toBeTruthy();
    await setNoauto(analyst, 0.55);

    // Шаг 1. Очередь маркетолога.
    const page = await marketerCtx.newPage();
    await uiLogin(page, 'marketer', marketerPassword);
    await gotoStable(page, '/queue', queueTable);
    const table = queueTable(page);
    await expect(page.getByRole('region', { name: 'Ожидаемая потеря за 30 дней' })).toContainText(
      money('3 599,07'),
    );
    const before = await customers(marketerCtx.request);
    expect(before[0]?.id).toBe(9);
    const lossBefore = before.find((r) => r.id === 1)!;
    expect(lossBefore.level).toBe('medium');

    // Шаг 2. Карточка №9.
    await table.locator('tbody tr').first().click();
    await expect(page).toHaveURL(/\/queue\/9$/);
    await expect(page.getByText(money('3 189,10')).first()).toBeVisible();
    await gotoStable(page, '/queue', queueTable);

    // Шаг 3. «Сводка»: где теряем деньги, по причинам.
    await test.step('Сводка: где теряем деньги', async () => {
      await gotoStable(page, '/summary', (p) => p.getByRole('heading', { name: 'Где теряем деньги' }));
      await expect(page.getByText('Это не доказанная причинность.')).toBeVisible();
      // Клик по причине открывает очередь только с её клиентами (на наборе организаторов susp — двое).
      await page.getByRole('link', { name: /Услуга приостановлена: показать клиентов/ }).click();
      await expect(page).toHaveURL(/\/queue\?reason=susp$/);
      await expect(queueTable(page).locator('tbody tr')).toHaveCount(2);
      await expect(queueTable(page).getByText(/по причине:/)).toHaveCount(2);
      await gotoStable(page, '/queue', queueTable);
    });

    // Шаг 4. Аналитик меняет вес — очередь обновляется без перезагрузки.
    let reloaded = false;
    page.on('load', () => {
      reloaded = true;
    });
    await setNoauto(analyst, 0.85);
    await expect(page.getByRole('region', { name: 'Ожидаемая потеря за 30 дней' })).toContainText(
      money('3 805,22'),
      { timeout: 15_000 },
    );
    expect(reloaded, 'страница не перезагружалась').toBe(false);
    const high = (await customers(marketerCtx.request)).find((r) => r.id === 1)!;
    expect(high.level).toBe('high');
    expect(high.score).toBeCloseTo(84.75, 1);

    // Устойчивость очереди к весам видит аналитик на «О модели».
    await test.step('О модели: устойчивость очереди к весам', async () => {
      // Аналитик уже вошёл через apiLogin: cookie общие с окнами контекста, экранный вход (он ждёт очередь маркетолога) не нужен.
      const analystPage = await analystCtx.newPage();
      await gotoStable(analystPage, '/about', (p) => p.getByRole('heading', { name: 'Насколько устойчива очередь' }));
      await expect(analystPage.getByRole('region', { name: 'Устойчивость очереди к весам' })).toBeVisible();
      await analystPage.close();
    });

    // Шаг 5. Эмулятор: автопродление №1 — потеря падает (~375 €).
    // Эффект ~375 € — при весе по умолчанию 0,55 (при 0,85 он 579,67 €), поэтому вес возвращаем.
    await setNoauto(analyst, 0.55);
    await expect
      .poll(async () => (await customers(marketerCtx.request)).find((r) => r.id === 1)?.level, {
        timeout: 15_000,
      })
      .toBe('medium');
    const raised = (await customers(marketerCtx.request)).find((r) => r.id === 1)!;
    const act = await mutate(analyst, 'post', '/api/v1/source/emulator/actions', { actions: [{ customerId: 1, actionId: 'enable_autoprolong' }] });
    expect(act.ok(), 'enable_autoprolong').toBeTruthy();
    await expect
      .poll(async () => (await customers(marketerCtx.request)).find((r) => r.id === 1)?.level, {
        timeout: 15_000,
      })
      .toBe('low');
    const after = (await customers(marketerCtx.request)).find((r) => r.id === 1)!;
    const drop = raised.expectedLossCents - after.expectedLossCents;
    expect(drop).toBeGreaterThan(30_000);
    expect(drop).toBeLessThan(40_000);

    // Шаг 6. Залп 10 000 событий: живые графики во время прогона и сверка с пересчётом с нуля («Источник»).
    await test.step('Источник: залп 10 000 событий, графики, согласованность 0 центов', async () => {
      // Аналитик уже вошёл через apiLogin: cookie общие с окнами контекста, экранный вход (он ждёт очередь маркетолога) не нужен.
      const analystPage = await analystCtx.newPage();
      await gotoStable(analystPage, '/source', (p) => p.getByRole('region', { name: 'Поток событий' }));
      const panel = analystPage.getByRole('region', { name: 'Поток событий' });
      await expect(panel.getByLabel('Скорость')).toHaveValue('Как можно быстрее');
      await panel.getByLabel('Залп, событий').click();
      await analystPage.getByRole('option', { name: /^10\s000$/ }).click();
      await panel.getByRole('button', { name: /Прогнать .*10.000.* событий/ }).click();
      await expect(panel.getByRole('region', { name: 'Живые графики потока' })).toBeVisible({ timeout: 60_000 });
      await expect(panel.getByRole('img', { name: /^Очередь приёма за последние 60 секунд/ })).toBeVisible();
      await expect(panel.getByRole('status').filter({ hasText: /Согласованность: пересчёт с нуля совпал \(разница 0\s¢/ })).toBeVisible({
        timeout: 180_000,
      });
      await expect(panel.getByText('Ошибок 0 · потерянных 0 · согласованность 0 центов')).toBeVisible();
      await expect(panel.getByRole('region', { name: 'Итог прогона' })).toContainText('Применено событий');
      // Кнопка «Мониторинг» есть только у ролей с правом source.view: маркетологу её не показывают.
      await expect(analystPage.getByRole('link', { name: 'Мониторинг' })).toHaveAttribute('href', '/grafana/');
      await analystPage.close();
    });

    // Шаг 7. /metrics = сводка.
    const summary = await (await marketerCtx.request.get('/api/v1/analytics/summary')).json();
    const metrics = await marketerCtx.request.get('/metrics');
    expect(metrics.ok(), '/metrics').toBeTruthy();
    expect(await metrics.text()).toMatch(
      new RegExp(`^churn_expected_loss_cents\\S* ${summary.expectedLossCents}(\\.0+)?$`, 'm'),
    );
  } finally {
    // Возврат стенда в начальное состояние, чтобы следующий прогон начинался чисто.
    // Запросы с тайм-аутом, ошибки очистки логируются, а не глотаются; закрытие контекстов не держит тест.
    const cleanupTimeout = 10_000;
    const csrf = { 'X-CSRF-Token': csrfTokens.get(analyst) ?? '' };
    // Сначала сброс (запросы идут через analystCtx), и только потом закрытие контекстов.
    const cleanup = await Promise.allSettled([
      analyst.post('/api/v1/source/emulator/actions', {
        data: { actions: [{ customerId: 0, actionId: 'reset' }] },
        headers: csrf,
        timeout: cleanupTimeout,
      }),
      analyst.patch('/api/v1/model/settings', {
        data: { weights: { noauto: 0.55 } },
        headers: csrf,
        timeout: cleanupTimeout,
      }),
    ]);
    const closed = await Promise.allSettled([marketerCtx.close(), analystCtx.close()]);
    for (const r of [...cleanup, ...closed]) {
      if (r.status === 'rejected') console.warn('очистка стенда:', r.reason);
    }
  }
});

// ADR-0004: цикла «взять в работу» в интерфейсе нет. Карточка клиента 9 — причины, потеря, действие с ценой.
test('демо: карточка без CRM-цикла', async ({ browser, baseURL }) => {
  test.skip(!juryPassword, 'нужен E2E_JURY_PASSWORD');
  const ctx = await browser.newContext({ baseURL });
  try {
    const page = await ctx.newPage();
    await uiLogin(page, 'jury', juryPassword);
    await gotoStable(page, '/queue/9', (p) => p.getByRole('heading', { name: 'Клиент 9' }));
    await expect(page.getByText(money('3 189,10')).first()).toBeVisible();
    for (const name of ['Отметить: взято в работу', 'Взять в работу', 'Связались', 'Отложить до…']) {
      await expect(page.getByRole('button', { name })).toHaveCount(0);
    }
    await expect(page.getByRole('region', { name: 'Работа с клиентом' })).toHaveCount(0);
    await expect(page.getByRole('region', { name: 'После взятия в работу' })).toHaveCount(0);
    await expect(page.getByRole('navigation', { name: 'Разделы' })).not.toContainText('Задачи');
    await page.goto('/tasks', { waitUntil: 'domcontentloaded' });
    await expect(page).toHaveURL(/\/queue$/);
  } finally {
    await ctx.close();
  }
});
