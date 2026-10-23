# backend — «Контур удержания»

Go + PostgreSQL. Контракт — `../api/openapi.yaml`; правила — [AGENTS.md](AGENTS.md).

## Запуск локально

```bash
docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=ci -e POSTGRES_USER=ci -e POSTGRES_DB=ci postgres:16-alpine
export DATABASE_URL='postgres://ci:ci@localhost:5432/ci?sslmode=disable'
go run ./cmd/api
```

Миграции (`migrations/`, goose) применяются при старте. Проверка: `curl localhost:8080/healthz`, Swagger UI — `http://localhost:8080/api/v1/docs`.

## Переменные окружения

| Переменная | По умолчанию | Назначение |
|---|---|---|
| `DATABASE_URL` | — (обязательна) | строка подключения PostgreSQL |
| `HTTP_ADDR` | `:8080` | адрес сервера (старое имя `ADDR` тоже читается) |
| `APP_VERSION` | `dev` | ответ `GET /version` |
| `TEST_DATABASE_URL` | — | сервер PostgreSQL для тестов; каждый тест создаёт свою базу. Без неё тесты с базой пропускаются |

## Команды

- `make generate` — копирует контракт в `internal/platform/docs/` и генерирует `internal/platform/api/gen/` (oapi-codegen); CI требует, чтобы после него не было диффа.
- `make check` — gofmt, go vet, golangci-lint (границы модулей), тесты.
- `make test-int` — тесты с тегом `integration`.

## Устройство

- `cmd/api` — конфиг и запуск; `internal/app` — сборка сервера (его же поднимает `internal/apitest`).
- `internal/{auth,source,risk,analytics,model}` — модули; пока заглушки с ответом 501 `problem+json`. Каждый реализует свою часть сгенерированного интерфейса и не импортирует другие модули.
- `internal/platform/{clock,db,httpx,events,docs,webui}` — общая обвязка без доменной логики.
- Тест эндпоинта: `s := apitest.New(t)`, дальше вызовы через `s.Client`.
