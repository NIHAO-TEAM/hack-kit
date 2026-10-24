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

test('демо: очередь → карточка → потеря по причинам → вес → устойчивость → автопродление → поток событий → /metrics', async ({ browser, baseURL }) => {
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

    // Шаг 3. «Сводка»: где теряем деньги, по причинам.
    await test.step('Сводка: где теряем деньги', async () => {
      await page.goto('/summary');
      await expect(page.getByRole('heading', { name: 'Где теряем деньги' })).toBeVisible();
      await expect(page.getByText('Это не доказанная причинность.')).toBeVisible();
      await page.goto('/queue');
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
      const analystPage = await analystCtx.newPage();
      await uiLogin(analystPage, 'analyst', analystPassword);
      await analystPage.goto('/about');
      await expect(analystPage.getByRole('heading', { name: 'Насколько устойчива очередь' })).toBeVisible();
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

    // Шаг 6. Поток 2 500 событий и сверка с пересчётом с нуля («Источник»).
    await test.step('Источник: поток 2 500 событий, согласованность 0 центов', async () => {
      const analystPage = await analystCtx.newPage();
      await uiLogin(analystPage, 'analyst', analystPassword);
      await analystPage.goto('/source');
      const panel = analystPage.getByRole('region', { name: 'Поток событий' });
      await panel.getByLabel('Скорость').click();
      await analystPage.getByRole('option', { name: 'Без пауз' }).click();
      await panel.getByRole('button', { name: /Прогнать .*2.500.* событий/ }).click();
      await expect(panel.getByRole('status').filter({ hasText: /Согласованность: пересчёт с нуля совпал \(разница 0\s¢/ })).toBeVisible({
        timeout: 180_000,
      });
      await expect(panel.getByRole('region', { name: 'Итог прогона' })).toContainText('Применено событий');
      await analystPage.close();
    });

    // Шаг 7. Инженерия: ссылка «Мониторинг» ведёт на Grafana (сама Grafana на стенде раннера может отсутствовать).
    await test.step('Ссылка «Мониторинг» ведёт на /grafana/', async () => {
      await expect(page.getByRole('link', { name: 'Мониторинг' })).toHaveAttribute('href', '/grafana/');
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
    await mutate(analyst, 'post', '/api/v1/source/emulator/actions', { actions: [{ customerId: 0, actionId: 'reset' }] })
      .catch(() => undefined);
    await setNoauto(analyst, 0.55).catch(() => undefined);
    await marketerCtx.close();
    await analystCtx.close();
  }
});

// ADR-0004: цикла «взять в работу» в интерфейсе нет. Карточка клиента 9 — причины, потеря, действие с ценой.
test('демо: карточка без CRM-цикла', async ({ browser, baseURL }) => {
  test.skip(!juryPassword, 'нужен E2E_JURY_PASSWORD');
  const ctx = await browser.newContext({ baseURL });
  try {
    const page = await ctx.newPage();
    await uiLogin(page, 'jury', juryPassword);
    await page.goto('/queue/9');
    await expect(page.getByRole('heading', { name: 'Клиент 9' })).toBeVisible();
    await expect(page.getByText(money('3 189,10')).first()).toBeVisible();
    for (const name of ['Отметить: взято в работу', 'Взять в работу', 'Связались', 'Отложить до…']) {
      await expect(page.getByRole('button', { name })).toHaveCount(0);
    }
    await expect(page.getByRole('region', { name: 'Работа с клиентом' })).toHaveCount(0);
    await expect(page.getByRole('region', { name: 'После взятия в работу' })).toHaveCount(0);
    await expect(page.getByRole('navigation', { name: 'Разделы' })).not.toContainText('Задачи');
    await page.goto('/tasks');
    await expect(page).toHaveURL(/\/queue$/);
  } finally {
    await ctx.close();
  }
});
