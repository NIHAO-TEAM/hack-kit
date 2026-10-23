import { expect, test, type APIRequestContext, type Page } from '@playwright/test';

// Сценарий — docs/demo-script.md. Учётки и пароли только из окружения.
const marketerPassword = process.env.E2E_MARKETER_PASSWORD ?? '';
const analystPassword = process.env.E2E_ANALYST_PASSWORD ?? '';

const NBSP = ' ';
const money = (euros: string) => `${euros}${NBSP}€`;

interface Row {
  id: number;
  level: string;
  score: number | null;
  expectedLossCents: number;
}

async function apiLogin(request: APIRequestContext, login: string, password: string) {
  const res = await request.post('/api/v1/auth/login', { data: { login, password } });
  expect(res.ok(), `вход ${login}`).toBeTruthy();
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
  const res = await request.patch('/api/v1/model/settings', { data: { weights: { noauto: value } } });
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
    const reset = await analyst.post('/api/v1/source/emulator/actions', {
      data: { actions: [{ customerId: 0, actionId: 'reset' }] },
    });
    expect(reset.ok(), 'reset эмулятора').toBeTruthy();
    await setNoauto(analyst, 0.55);

    // Шаг 1. Очередь маркетолога.
    const page = await marketerCtx.newPage();
    await uiLogin(page, 'marketer', marketerPassword);
    await page.goto('/queue');
    const table = page.getByRole('table', { name: 'Клиенты под риском оттока' });
    await expect(page.getByRole('region', { name: 'Ожидаемая потеря за 30 дней' })).toContainText(
      money('3 879,73'),
    );
    await expect(table.locator('tbody tr').first()).toContainText('9');
    const before = await customers(marketerCtx.request);
    expect(before[0]?.id).toBe(9);
    const lossBefore = before.find((r) => r.id === 1)!;
    expect(lossBefore.level).toBe('medium');

    // Шаг 2. Карточка №9.
    await table.locator('tbody tr').first().click();
    await expect(page).toHaveURL(/\/queue\/9$/);
    await expect(page.getByText(money('3 459,27')).first()).toBeVisible();
    await page.goto('/queue');

    // Шаг 3. Аналитик меняет вес — очередь обновляется без перезагрузки.
    let reloaded = false;
    page.on('load', () => {
      reloaded = true;
    });
    await setNoauto(analyst, 0.85);
    await expect(page.getByRole('region', { name: 'Ожидаемая потеря за 30 дней' })).toContainText(
      money('4 080,42'),
      { timeout: 15_000 },
    );
    expect(reloaded, 'страница не перезагружалась').toBe(false);
    const raised = (await customers(marketerCtx.request)).find((r) => r.id === 1)!;
    expect(raised.level).toBe('high');
    expect(raised.score).toBeCloseTo(85.15, 1);

    // Шаг 4. Эмулятор: автопродление №1 — потеря падает (~365 €).
    const act = await analyst.post('/api/v1/source/emulator/actions', {
      data: { actions: [{ customerId: 1, actionId: 'enable_autoprolong' }] },
    });
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
    await analyst
      .post('/api/v1/source/emulator/actions', {
        data: { actions: [{ customerId: 0, actionId: 'reset' }] },
      })
      .catch(() => undefined);
    await setNoauto(analyst, 0.55).catch(() => undefined);
    await marketerCtx.close();
    await analystCtx.close();
  }
});
