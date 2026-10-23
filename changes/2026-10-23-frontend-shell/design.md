# Дизайн: оболочка фронта

## Подход

- Структура FSD по скиллу `frontend-architecture`: `app/` (провайдеры Mantine, Query, Router; `routes/`), `pages/` (`login`, `queue`, `summary`, `model`, `source` — заглушки), `entities/session` (query `me`, `login`/`logout` мутации, `can(role, right)`), `shared/api` (`openapi-fetch` клиент с `credentials: 'include'` и `X-CSRF-Token` из ответа `login`/`me`), `shared/ui/states` (пять состояний экрана).
- **Права на фронте** — только для вида: `can()` по роли из `me`; настоящая проверка на сервере (`auth-sessions`).
- **Моки:** MSW-обработчики для `auth/*`, `customers`, `customers/{id}`, `analytics/summary`, `model/settings` — ответы собираются из копии `cases/analizator/data.json` и примеров спеки; три учётки моков: `marketer`, `analyst`, `jury`, пароль `demo` (только в моках).
- **Прокси:** `pnpm dev` проксирует `/api` на `http://localhost:8080`.
- **Сборка:** `vite build` → `frontend/dist` (её забирает `Dockerfile` из `compose-stand`).
- **Тексты** — по-русски; время показывается в МСК с подписью.

## Отброшенные варианты

| Вариант | Почему нет |
|---|---|
| Ждать ADR-0002 до каркаса | экраны этапа 4 не на чем верстать; каркас не зависит от палитры |
| Свои цвета до ADR-0002 | придётся переделывать; правило «только система» (`frontend/AGENTS.md`) |

## Допущения

- [ПРЕДПОЛОЖЕНО] Стартовая страница: маркетолог — «Очередь», аналитик — «Модель».
- [ПРОВЕРЕНО] Роли и права — `changes/2026-10-23-auth-sessions/design.md`.

## Фронт

- Макет: не нужен (стандартные компоненты, без своей вёрстки). Состояния: загрузка, пусто, ошибка, нет связи, нет прав — компонентом `shared/ui/states`.
- Новые компоненты `shared/ui`: пять состояний экрана; витрина `/dev/ui` с ними.
- Роль и право: вход — все; «Модель», «Источник» — `analyst`, `jury`, `admin`.

## Риски

- Контракт из `churn-foundation` ещё может уточняться — типы перегенерируются `pnpm api:types`, ручных типов API нет.
