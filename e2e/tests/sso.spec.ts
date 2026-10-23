import { createHmac } from 'node:crypto';
import { expect, test, type Page } from '@playwright/test';

// Вход через Keycloak: пароль + TOTP. Стенд — Compose с профилем `sso` или прод с Keycloak.
// Пароль и секрет TOTP — только из окружения, в лог не печатаем.
// Секрет TOTP в realm — сырая строка, ключ — её байты UTF-8 (RFC 6238, HmacSHA1, 6 цифр, 30 с).
const login = process.env.E2E_SSO_LOGIN ?? 'secure';
const password = process.env.E2E_SSO_PASSWORD ?? '';
const totpSecret = process.env.E2E_SSO_TOTP_SECRET ?? '';

function totp(secret: string, stepOffset = 0): string {
  const counter = BigInt(Math.floor(Date.now() / 30_000) + stepOffset);
  const msg = Buffer.alloc(8);
  msg.writeBigUInt64BE(counter);
  const h = createHmac('sha1', Buffer.from(secret, 'utf8')).update(msg).digest();
  const o = h[h.length - 1] & 0x0f;
  const bin = h.readUInt32BE(o) & 0x7fffffff;
  return String(bin % 1_000_000).padStart(6, '0');
}

async function startSSO(page: Page) {
  await page.goto('/login');
  await page.getByRole('link', { name: 'Войти через SSO' }).click();
  await page.locator('#username').fill(login);
  await page.locator('#password').fill(password);
  await page.locator('#kc-login').click();
  await page.locator('#otp').waitFor();
}

test.describe('вход через SSO', () => {
  test.skip(!password || !totpSecret, 'нужны E2E_SSO_PASSWORD и E2E_SSO_TOTP_SECRET');

  test('верный код TOTP: очередь с ролью из claim', async ({ page, request }) => {
    await startSSO(page);
    await page.locator('#otp').fill(totp(totpSecret));
    await page.locator('#kc-login').click();

    await expect(page.getByRole('table', { name: 'Клиенты под риском оттока' })).toBeVisible();
    const me = await page.request.get('/api/v1/auth/me');
    expect(me.status()).toBe(200);
    expect((await me.json()).role).toBe('marketer');
    // чужой контекст без cookie сессии не входит
    expect((await request.get('/api/v1/auth/me')).status()).toBe(401);
  });

  test('неверный код TOTP: ошибка Keycloak, сессии нет', async ({ page }) => {
    await startSSO(page);
    // код из окна далеко за допуском Keycloak (±1 шаг)
    await page.locator('#otp').fill(totp(totpSecret, 10));
    await page.locator('#kc-login').click();

    await expect(page.locator('#input-error-otp-code, .kc-feedback-text, .alert-error')).toBeVisible();
    await expect(page).toHaveURL(/\/sso\/realms\/churn\//);
    const me = await page.request.get('/api/v1/auth/me');
    expect(me.status()).toBe(401);
  });
});
