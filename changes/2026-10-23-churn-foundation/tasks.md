# Задачи: каркас «Контура удержания»

Каждая задача проверяема: в конце строки — чем проверить.

## 1. Контракт и данные

- [x] 1.1 `api/openapi.yaml`: пути и схемы по таблице `design.md`, `securitySchemes`, `problem+json`, `int64` для денег; `/healthz`, `/version`, `/ping` без изменений — проверка: `npx @redocly/cli lint api/openapi.yaml` без ошибок; `contract-diff.sh` не видит удалений старых путей
- [x] 1.2 `oapi-codegen` (версия фиксирована в `tools.go` или `go.mod`), `make generate` пишет `internal/platform/api/gen/` — проверка: `make generate && git diff --exit-code`
- [x] 1.3 Миграции `goose`: таблицы организаторов 1:1 из `cases/analizator/001_schema.up.sql` + `snapshots`, `model_settings`, `users`, `sessions`; применяются при старте — проверка: интеграционный тест «миграции вверх и вниз на чистой базе»
- [x] 1.4 Проверить, что CI ловит дифф генерации; если нет — запись в «Найдено по ходу» — проверка: чтение `.github/workflows/ci.yml`

## 2. Логика

- [x] 2.1 `internal/platform/{clock,db,httpx,events}`: часы, пул pgx, `problem+json` с `correlation_id`, шина событий в процессе — проверка: unit-тесты `httpx` (формат ошибки) и `events` (подписчик получает событие)
- [x] 2.2 Заглушки `internal/{auth,source,risk,analytics,model}/http.go` — все операции отвечают 501 `problem+json` — проверка: тест «каждый путь спеки, кроме открытых, отвечает 501»
- [x] 2.3 `cmd/api` собирает сервер из сгенерированного интерфейса и модулей; `withRequestID` сохранён; конфиг из окружения (`DATABASE_URL`, `HTTP_ADDR`) — проверка: старые тесты `cmd/api/*_test.go` зелёные
- [x] 2.4 `GET /api/v1/openapi.yml`, `/openapi.json`, `/docs` — спека и Swagger UI из `embed` — проверка: тест «`/api/v1/docs` отвечает 200 HTML без входа; `/openapi.yml` = файл контракта»
- [ ] 2.5 `apitest.New(t)` — API на отдельной базе (testcontainers или `TEST_DATABASE_URL`) + сгенерированный клиент — проверка: тест-пример через клиент
- [ ] 2.6 depguard: модули не импортируют друг друга, `platform` не импортирует модули — проверка: `make check`

## 3. Экран

- нет

## 4. Готовность

- [ ] 4.1 `make check` зелёный, `make test-int` зелёный
- [ ] 4.2 Сценарии из `specs/` проходят на локальном стенде
- [ ] 4.3 `backend/README` или раздел README: как запустить API локально, переменные окружения
