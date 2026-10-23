import { expect, test, type APIRequestContext } from '@playwright/test';

// Дымовая проверка прода после выката (deploy-smoke.yml). Данные прода не меняет
// и не привязана к суммам: только доступность, закрытые пути и согласованность чисел.
// Пароль `jury` — только из окружения; ответы с данными в лог не печатаем.
const juryPassword = process.env.E2E_JURY_PASSWORD ?? '';

interface Row {
  id: number;
  expectedLossCents: number;
}

test.describe('открытая часть', () => {
  test('главная открывается', async ({ page }) => {
    const res = await page.goto('/');
    expect(res?.ok(), 'ответ главной').toBeTruthy();
    await expect(page.locator('#root')).not.toBeEmpty();
  });

  test('Swagger на /api/v1/docs', async ({ page }) => {
    const res = await page.goto('/api/v1/docs');
    expect(res?.ok(), 'ответ /api/v1/docs').toBeTruthy();
    await expect(page.locator('body')).toContainText(/swagger|openapi/i);
  });

  test('пути с данными без входа отвечают 401', async ({ request }) => {
    for (const path of ['/api/v1/customers', '/api/v1/analytics/summary', '/api/v1/model/settings']) {
      const res = await request.get(path);
      expect(res.status(), path).toBe(401);
    }
  });

  test('/metrics без токена закрыт', async ({ request }) => {
    for (const path of ['/metrics', '/metics']) {
      const res = await request.get(path);
      expect([401, 403], path).toContain(res.status());
    }
  });

  test('Grafana за входом', async ({ page }) => {
    await page.goto('/grafana/');
    await expect(page).toHaveURL(/\/grafana\/login/);
    await expect(page.locator('input[name="user"]')).toBeVisible();
  });
});

async function allRows(request: APIRequestContext): Promise<{ rows: Row[]; version: number }> {
  const rows: Row[] = [];
  let version = 0;
  for (let offset = 0; ; offset += 200) {
    const res = await request.get(`/api/v1/customers?limit=200&offset=${offset}`);
    expect(res.ok(), 'список клиентов').toBeTruthy();
    const page = (await res.json()) as { items: Row[]; total: number; version: number };
    rows.push(...page.items);
    version = page.version;
    if (rows.length >= page.total || page.items.length === 0) break;
  }
  return { rows, version };
}

test.describe('со входом jury', () => {
  test.skip(!juryPassword, 'нет E2E_JURY_PASSWORD: только открытая часть');

  test('очередь, карточка и сводка', async ({ page }) => {
    await page.goto('/login');
    await page.getByLabel('Логин').fill('jury');
    await page.getByLabel('Пароль').fill(juryPassword);
    await page.getByRole('button', { name: 'Войти' }).click();

    const table = page.getByRole('table', { name: 'Клиенты под риском оттока' });
    await expect(table).toBeVisible();
    const first = table.locator('tbody tr').first();
    await expect(first).toBeVisible();

    await first.click();
    await expect(page).toHaveURL(/\/queue\/\d+$/);
    await expect(page.getByText('Ожидаемая потеря').first()).toBeVisible();

    // Сводка = сумме очереди. Пересчёт может пойти между запросами — повторяем, пока версии равны.
    await expect
      .poll(
        async () => {
          const before = await allRows(page.request);
          const sumRes = await page.request.get('/api/v1/analytics/summary');
          expect(sumRes.ok(), 'сводка').toBeTruthy();
          const summary = (await sumRes.json()) as { expectedLossCents: number; version: number };
          expect(before.rows.length, 'очередь не пуста').toBeGreaterThan(0);
          if (summary.version !== before.version) return 'версии разошлись';
          const sum = before.rows.reduce((s, r) => s + r.expectedLossCents, 0);
          return sum === summary.expectedLossCents ? 'ok' : `сумма очереди ${sum} ≠ сводка ${summary.expectedLossCents}`;
        },
        { timeout: 20_000 },
      )
      .toBe('ok');
  });
});
