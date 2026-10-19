# Дизайн: конфиг Playwright

## Подход

`defineConfig` из `@playwright/test`: `testDir: './tests'`, `use.baseURL = process.env.E2E_BASE_URL`, `retries: process.env.CI ? 1 : 0`, один проект `{ name: 'firefox', use: { ...devices['Desktop Firefox'] } }`. Chromium и WebKit не подключаются.

## Отброшенные варианты

| Вариант | Почему нет |
|---|---|
| Конфиг в корне репозитория | Правила e2e живут в `e2e/` |
| Все три браузера | Chromium в нашей среде запрещён, WebKit не нужен для демо |

## Допущения

- [ПРЕДПОЛОЖЕНО] Playwright 1.4x+ — API `defineConfig` и `devices` стабильны; проверить по документации актуальной версии.

## Риски

- Конфиг не компилируется в CI (TS в `e2e/` никто не собирает) — проверка: синтаксис по документации, `node --check` неприменим к TS; достаточно ревью.
