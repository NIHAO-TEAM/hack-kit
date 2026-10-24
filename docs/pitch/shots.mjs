// Скриншоты продукта для слайдов: node docs/pitch/shots.mjs <база> <логин> <пароль> <папка>.
// Пароль — из окружения стенда, в репозиторий не пишется. Стенд — свежий сид организаторов.
import { createRequire } from 'node:module';
// playwright стоит глобально (среда пахана) или в e2e/: ищем через require, он читает NODE_PATH.
const { chromium } = createRequire(import.meta.url)('playwright');
import { mkdirSync } from 'node:fs';

const [base, login, password, out] = process.argv.slice(2);
if (!base || !login || !password || !out) {
  console.error('нужно: <база> <логин> <пароль> <папка>');
  process.exit(2);
}
mkdirSync(out, { recursive: true });

const browser = await chromium.launch();
const ctx = await browser.newContext({
  viewport: { width: 1280, height: 800 },
  deviceScaleFactor: 2,
  colorScheme: 'light',
  locale: 'ru-RU',
  timezoneId: 'Europe/Moscow',
});
const page = await ctx.newPage();

async function shot(name, path, ready) {
  if (path) await page.goto(base + path);
  if (ready) await ready();
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(800);
  // Только рабочая область без бокового меню и шапки: на слайде текст крупнее.
  const main = page.locator('main').first();
  if (await main.count()) await main.screenshot({ path: `${out}/${name}.png` });
  else await page.screenshot({ path: `${out}/${name}.png` });
  console.log(`${out}/${name}.png`);
}

await shot('login', '/login', () => page.getByLabel('Логин').waitFor());
await page.getByLabel('Логин').fill(login);
await page.getByLabel('Пароль').fill(password);
await page.getByRole('button', { name: 'Войти' }).click();
await page.waitForURL((u) => !u.pathname.startsWith('/login'));

await shot('queue', '/queue', () => page.getByRole('table').first().waitFor());
await shot('card', '/queue/9', () => page.getByRole('heading').first().waitFor());
await shot('summary', '/summary');
await shot('model', '/model');
await shot('about', '/about');
await shot('source', '/source');

await browser.close();
