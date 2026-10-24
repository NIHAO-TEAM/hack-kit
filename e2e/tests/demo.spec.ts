import { expect, test, type APIRequestContext, type Page } from '@playwright/test';

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

test('демо: очередь → карточка → вес → автопродление → /metrics', async ({ browser, baseURL }) => {
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
    await page.goto('/queue');
    const table = page.getByRole('table', { name: 'Клиенты под риском оттока' });
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
    await page.goto('/queue');

    // Шаг 3. Аналитик меняет вес — очередь обновляется без перезагрузки.
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

    // Шаг 4. Эмулятор: автопродление №1 — потеря падает (~375 €).
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

    // Шаг 5. /metrics = сводка.
    const summary = await (await marketerCtx.request.get('/api/v1/analytics/summary')).json();
    const metrics = await marketerCtx.request.get('/metrics');
    expect(metrics.ok(), '/metrics').toBeTruthy();
    expect(await metrics.text()).toMatch(
      new RegExp(`^churn_expected_loss_cents\\S* ${summary.expectedLossCents}(\\.0+)?$`, 'm'),
    );
  } finally {
    // Возврат стенда в начальное состояние, чтобы следующий прогон начинался чисто.
    await mutate(analyst, 'post', '/api/v1/source/emulator/actions', { actions: [{ customerId: 0, actionId: 'reset' }] })
      .catch(() => undefined);
    await setNoauto(analyst, 0.55).catch(() => undefined);
    await marketerCtx.close();
    await analystCtx.close();
  }
});

interface TaskItem {
  id: number;
  customerId: number;
  status: string;
  outcome?: string;
  version: number;
}

async function tasksOf(request: APIRequestContext, customerId: number): Promise<TaskItem[]> {
  const res = await request.get('/api/v1/tasks?limit=200');
  expect(res.ok(), 'список задач').toBeTruthy();
  return ((await res.json()).items as TaskItem[]).filter((t) => t.customerId === customerId);
}

// Цикл удержания (шаг 5 демо, ADR-0004): «Отметить: взято в работу» → пополнение эмулятором → наблюдение → закрыть «Продлил».
// Экрана «Задачи» нет: вся цепочка в карточке клиента.
// Задачи сбросом эмулятора не стираются, поэтому старую открытую задачу клиента 9 закрываем до начала.
test('демо: отметить «взято в работу» → пополнение → наблюдение → закрыть «Продлил»', async ({ browser, baseURL }) => {
  test.skip(!juryPassword, 'нужен E2E_JURY_PASSWORD');
  const ctx = await browser.newContext({ baseURL });
  const api = ctx.request;
  try {
    await apiLogin(api, 'jury', juryPassword);
    const reset = await mutate(api, 'post', '/api/v1/source/emulator/actions', { actions: [{ customerId: 0, actionId: 'reset' }] });
    expect(reset.ok(), 'reset эмулятора').toBeTruthy();
    for (const t of (await tasksOf(api, 9)).filter((x) => x.status !== 'closed')) {
      const closed = await mutate(api, 'patch', `/api/v1/tasks/${t.id}`, { version: t.version, status: 'closed', outcome: 'not_needed' });
      expect(closed.ok(), 'закрытие старой задачи клиента 9').toBeTruthy();
    }
    const lossTaken = (await customers(api)).find((r) => r.id === 9)!.expectedLossCents;

    // Отметить в карточке: срок и ответственный в интерфейсе не показываются.
    const page = await ctx.newPage();
    await page.goto('/queue/9');
    await page.getByRole('button', { name: 'Отметить: взято в работу' }).click();
    await expect(page.getByRole('button', { name: 'Закрыть', exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Связались' })).toHaveCount(0);

    // «Источник»: эмулятор пополняет баланс клиента 9.
    await page.goto('/source');
    const emulator = page.getByRole('region', { name: 'Эмулятор биллинга' });
    await emulator.getByLabel('Номер клиента').fill('9');
    await emulator.getByLabel('Сумма, €').fill('529,84');
    await emulator.getByRole('button', { name: 'Пополнить' }).click();

    // Карточка: наблюдение биллинга с подписью «после, а не благодаря», потеря меньше.
    await expect
      .poll(async () => (await customers(api)).find((r) => r.id === 9)!.expectedLossCents, {
        timeout: 20_000,
      })
      .toBeLessThan(lossTaken);
    await page.goto('/queue/9');
    const observed = page.getByRole('region', { name: 'После взятия в работу' });
    await expect(observed).toContainText(/Баланс пополнен на 529,84\s€/, { timeout: 20_000 });
    await expect(observed).toContainText('не доказывают, что помог именно контакт');

    // Закрыть задачу с итогом «Продлил».
    await page.getByRole('button', { name: 'Закрыть', exact: true }).click();
    await page.getByRole('combobox', { name: 'Итог' }).click();
    await page.getByRole('option', { name: 'Продлил' }).click();
    await page.getByRole('button', { name: 'Закрыть задачу' }).click();
    await expect(page.getByRole('button', { name: 'Отметить: взято в работу' })).toBeVisible();
    const last = (await tasksOf(api, 9)).find((t) => t.status === 'closed' && t.outcome === 'renewed');
    expect(last, 'закрытая задача с итогом renewed').toBeTruthy();
  } finally {
    await mutate(api, 'post', '/api/v1/source/emulator/actions', { actions: [{ customerId: 0, actionId: 'reset' }] })
      .catch(() => undefined);
    await ctx.close();
  }
});
