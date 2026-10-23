# Дизайн: каркас «Контура удержания»

## Подход

**Генератор:** `oapi-codegen` (strict server поверх `net/http`, Go 1.22+ `ServeMux`). Сгенерированный код — `backend/internal/platform/api/gen/` (коммитится, руками не правится). Это фиксирует ADR-0005 — пункт «генератор» из `backend/AGENTS.md`; файл ADR не пишется в этом изменении (ADR пишет пахан с «да» человека), выбор записывается здесь [ПРЕДПОЛОЖЕНО].

**SQL:** `sqlc` (запросы — `queries.sql` модуля, код — `db/`); миграции — `goose`, файлы в `backend/migrations/`, версия — время создания; применяются при старте (`goose.Up` из `embed`).

**Пути контракта** (префикс `/api/v1`, кроме служебных):

| Путь | Чей модуль | Слой | Доступ |
|---|---|---|---|
| `POST /auth/login`, `GET /auth/me`, `POST /auth/logout` | auth | 1 | login открыт, остальное — сессия |
| `GET /customers` (фильтр `level`, `q`; сортировка `sort=id|email|balance|totalReplenished|score|expectedLoss`, `order=asc|desc`; `limit`, `offset`) | analytics | 1 | сессия |
| `GET /customers/{id}` (услуги, признаки с вкладом, деньги, действия с ценой) | analytics | 1 | сессия |
| `GET /analytics/summary` | analytics | 1 | сессия |
| `GET /analytics/churn-chart` (факт / прогноз по неделям) | analytics | 2 | сессия |
| `GET /model/settings`, `PATCH /model/settings?dryRun=true|false` | model | 1 | GET — сессия, PATCH — аналитик |
| `PUT /source/snapshot` | source | 1 | аналитик или сервисный токен |
| `GET /source/status` (версия, `asOf`, возраст, проверки качества) | source | 1 | сессия |
| `PATCH /source/services/{id}` (правка одной услуги: статус, `autoProlong`, `expiresAt`) | source | 1 | аналитик |
| `PATCH /source/accounts/{id}` (баланс, бонус) | source | 1 | аналитик |
| `POST /source/emulator/tick`, `POST /source/emulator/actions` | source | 1 | аналитик |
| `GET /events` (SSE: `{"type":"recalculated","version":N}`) | analytics | 1 | сессия |
| `GET /metrics`, `GET /metics` | analytics | 1 | токен сборщика или сессия |
| `GET /auth/oidc/login`, `GET /auth/oidc/callback` | auth | 2 | открыт |
| `GET /docs`, `GET /openapi.yml`, `GET /openapi.json` | platform | 1 | открыт |
| `GET /healthz`, `GET /version`, `GET /ping` (без префикса, как сейчас) | platform | — | открыт |

**Схемы:** `Account`, `Service`, `Transaction` — имена полей схемы организаторов (`price`, `paymentTerm`, `autoProlong`, `performed`, `quarter_year`); все деньги — `integer`, `format: int64`, евроценты. `Snapshot = {asOf?: date-time, accounts[], services[], transactions[]}`. `CustomerRow`, `CustomerDetail`, `Feature {id, value, weight, contribution, reason}`, `Action {id, title, effectCents, newScore, newLevel}`, `Summary {levels{high,medium,low,unscored,lost}, expectedLossCents, atRiskCents, lostCents, asOf, version}`, `ModelSettings {weights{}, thresholds{high,medium}, horizonDays, priceBasis: per_period|per_month, asOfMode: snapshot|server, version}`, `DryRunResult {levelChanges, expectedLossDeltaCents}`. Уровни: `high|medium|low|unscored|lost`.

**Безопасность в контракте:** `securitySchemes`: `cookieAuth` (сессия), `bearerAuth` (тот же токен сессии), `serviceToken` (источник), `metricsToken`. Ошибки — `application/problem+json` (RFC 9457) с `correlation_id` = `X-Request-Id`.

**Таблицы:** `accounts`, `services`, `transactions` — DDL 1:1 из `cases/analizator/001_schema.up.sql` (идентификаторы `bigint`, имена в кавычках); `snapshots(version bigint, as_of timestamptz, loaded_at, source)`; `model_settings(version bigint, body jsonb, updated_at, updated_by)`; `users(id, login unique, password_hash, role, totp_secret null, created_at)`; `sessions(token_hash primary key, user_id, csrf_token, expires_at)`.

**Заглушки модулей:** у каждого `internal/<модуль>/http.go` реализует свою часть сгенерированного интерфейса, возвращая 501. Так параллельные изменения этапа 2 правят только свой пакет.

**Точки подключения в `cmd/api` (ставятся здесь, дальше `cmd/api` никто не правит):**
- `auth.Middleware(next)` — проверка сессии и прав на каждый запрос; заглушка пропускает всё. Её наполняет `auth-sessions`.
- `auth.OnStart(ctx)` — создание учёток из окружения; заглушка ничего не делает.
- `source.OnStart(ctx)` — загрузка сида при пустой базе; заглушка ничего не делает. Её наполняет `snapshot-ingest`.
- `webui.Handler()` из `internal/platform/webui` — раздача собранного фронта на `/` с откатом на `index.html`; заглушка отдаёт текстовую страницу «фронт не собран». Её наполняет `compose-stand`.
- Шина `events` создаётся в `cmd/api` и передаётся модулям в конструкторах. `internal/platform/events` — шина в процессе (`Publish(topic)`, `Subscribe(topic)`), ей пользуются `snapshot-ingest` и `recalc-engine`.

**Swagger UI:** файлы `swagger-ui-dist` фиксированной версии вшиты через `embed` (`internal/platform/docs`).

## Отброшенные варианты

| Вариант | Почему нет |
|---|---|
| `ogen` | строже, но медленнее генерирует и сложнее с SSE и `text/plain` для `/metrics` |
| Контракт по частям в каждом изменении | `api/openapi.yaml` защищён; параллельные правки конфликтуют (`03_verdict.md`, правка 9) |
| Свои имена полей вместо схемы организаторов | жюри грузит `data.json` как есть |

## Допущения

- [ПРОВЕРЕНО] Поля и типы — `cases/analizator/001_schema.up.sql`, `data.json`.
- [ПРЕДПОЛОЖЕНО] Алиасы опечаток кейса (`pice`, `paymentTem`, `quater_year`, `pefomed`, `autoprolong`) в контракт не входят — их молча принимает `snapshot-ingest` при разборе.
- [ПРЕДПОЛОЖЕНО] `/metrics` в спеке описан как `text/plain; version=0.0.4`.
- [ПРОВЕРЕНО] Проверка «сгенерированный код без диффа» есть в `ci.yml` (`make generate` и `git diff --exit-code` в `backend/`); оговорка про неотслеживаемые файлы — в «Найдено по ходу» `proposal.md`.
- [ПРОВЕРЕНО] oapi-codegen v2.4.1 (+ runtime): генерирует модели, `std-http-server` (Go 1.22 `ServeMux`), клиент и встроенную спеку. Серверный интерфейс не strict: заглушки пишут `problem+json` напрямую через `httpx`; strict требует типизированного ответа на каждый статус, а модули этапа 2 сами решают, как отвечать (SSE, `text/plain`). Отступление от слова «strict server» выше.
- [ПРОВЕРЕНО] goose v3.24.3: версии новее требуют Go 1.25, а `go.mod` проекта — 1.24.

## Риски

- Контракт неполон — следующие изменения упрутся в новую правку защищённого файла. Митигация: перечень путей выше сверен с `03_verdict.md` и `01_solution.md`; недостающее — в «Найдено по ходу» и отдельным изменением.
