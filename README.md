# Контур удержания

Внутренний сервис aéza для маркетологов: показывает клиентов под риском оттока, причину простыми словами, ожидаемую потерю в евро и одно действие с ценой. Кейс и критерии: [docs/case.md](docs/case.md), решения: [docs/adr/](docs/adr/).

## Запуск

Нужен Docker с Compose. Три команды:

```sh
cp .env.example .env
# задайте в .env пароли и токены (случайное значение: openssl rand -hex 16)
docker compose up -d
```

Пустые `POSTGRES_PASSWORD` и `ADMIN_PASSWORD` — Compose или приложение не стартуют и называют переменную. База наружу не публикуется, снаружи доступен только порт приложения `8080`. Данные лежат в томе и переживают `docker compose down` (без `-v`).

## Что проверить за минуту

- `curl -fsS http://localhost:8080/healthz` отвечает `200`.
- `http://localhost:8080/` — веб-клиент (если фронт ещё не собран, страница «фронт не собран»).
- `http://localhost:8080/api/v1/docs` — Swagger по контракту `api/openapi.yaml`.
- Вход: логин `ADMIN_LOGIN` и пароль `ADMIN_PASSWORD` из `.env`; демо-учётки появляются, если заданы `DEMO_*_PASSWORD`.

## Переменные окружения

Полный список с комментариями — в [.env.example](.env.example).

| Переменная | Назначение |
|---|---|
| `POSTGRES_PASSWORD` | пароль базы (обязателен) |
| `ADMIN_LOGIN`, `ADMIN_PASSWORD` | администратор из окружения (пароль обязателен) |
| `DEMO_MARKETER_PASSWORD`, `DEMO_ANALYST_PASSWORD`, `DEMO_JURY_PASSWORD` | демо-учётки; пустой пароль — учётка не создаётся |
| `METRICS_TOKEN`, `SOURCE_TOKEN` | токены `/metrics` и приёма снимка источника |
| `SEED_ORGANIZERS` | загрузить набор организаторов при старте |

## Модель и архитектура

- [docs/model.md](docs/model.md) — признаки и веса, формула, уровни, деньги, допущения, чувствительность «вес ±0,1 → потеря».
- [docs/architecture.md](docs/architecture.md) — модули и потоки, деплой, безопасность, как встать в контур aéza.
- Экран «О модели» в приложении (`/about`) — то же с числами из настроек.
- Эталонный расчёт: `python3 -I tools/oracle/oracle.py --check`.

### Как добавить признак

Файл `backend/internal/risk/feature_<имя>.go` с реализацией `Feature` и `Register` в `init()`, правило в `docs/model-rules.md` и оракуле. Пошагово — в [docs/model.md](docs/model.md#как-добавить-признак). Пример готового изменения — PR `feature-bonus-locked` (признак с весом 0 по умолчанию).

## Безопасность

- Заголовки в каждом ответе (`httpx.SecurityHeaders`): CSP без `unsafe-inline` для скриптов, HSTS, `X-Content-Type-Options`, `Referrer-Policy`, `frame-ancestors 'none'`. Скрипты лежат файлами (`theme-init.js`, `init.js` Swagger UI), встроенных нет.
- Зависимости: `govulncheck` в `make check`, `pnpm audit --prod` в `pnpm check`.
- Логи не содержат паролей, токенов, логинов и балансов; образ работает не от root (`distroless:nonroot`).
- Секреты только из переменных окружения; в репозитории их нет.

## Дополнительно

Функции сверх обязательных:

Уже в `main`:

- Эталонный оракул модели и golden-тесты до цента (`tools/oracle`, `testdata/golden`).
- Действия с ценой: эффект в евро считается пересчётом на копии данных.
- Прод на Compose с TLS (Caddy) и деплоем из CI.

По плану этапов 4–7 (`docs/plan.md`), в README обновится после слияния: экран «Источник», эмулятор биллинга, SSE-обновления, метрики Prometheus, k3s и Helm, вход через Keycloak, график оттока, «что если», журнал действий.
- Образец «как добавить признак модели»: PR с `backend/internal/risk/feature_bonus_locked.go` (один файл признака + тест + строка оракула), см. `changes/archive/2026-10-23-feature-bonus-locked/`.

## Разработка

Правила репозитория — [AGENTS.md](AGENTS.md). Процесс работы команды и агентов (бывший README кита) — [hack-kit/OVERVIEW.md](hack-kit/OVERVIEW.md).
