# Дизайн: Compose и README

## Подход

- **Dockerfile** (корень, контекст — весь репозиторий):
  1. `node:<LTS>-alpine` + `corepack` → `pnpm install --frozen-lockfile` → `pnpm build` в `frontend/`, если скрипт есть; иначе пустой `dist/`.
  2. `golang:<версия из go.mod>` → копия `frontend/dist` в `backend/internal/platform/webui/dist` → `CGO_ENABLED=0 go build ./cmd/api`.
  3. `gcr.io/distroless/static:nonroot`, `USER nonroot`, `EXPOSE 8080`.
- **webui:** `//go:embed all:dist`; отдаёт файлы, неизвестный путь не под `/api/` и не служебный → `index.html`; заголовки `Cache-Control` для ассетов с хешем. В репозитории в `dist/` лежит только `.gitkeep` и `index.html`-заглушка.
- **Compose:**
  - `postgres:16-alpine`, `POSTGRES_PASSWORD` из `.env`, том `pgdata`, `healthcheck: pg_isready`, порт не публикуется;
  - `app`: `build: .`, `env_file: .env`, `depends_on: postgres: condition: service_healthy`, `ports: "8080:8080"`, `healthcheck` на `/healthz`;
  - `restart: unless-stopped` у обоих.
- **`.env.example`:** `DATABASE_URL`, `POSTGRES_PASSWORD`, `ADMIN_LOGIN`, `ADMIN_PASSWORD`, `DEMO_MARKETER_PASSWORD`, `DEMO_ANALYST_PASSWORD`, `DEMO_JURY_PASSWORD`, `METRICS_TOKEN`, `SOURCE_TOKEN`, `SEED_ORGANIZERS=true`, `SNAPSHOT_MAX_BYTES=536870912`, `COOKIE_SECURE=false`, `TRUSTED_PROXY=false`, `HTTP_ADDR=:8080`. Пароли и токены пустые, в комментарии — `openssl rand -hex 16`.
- **README:** запуск за 3 команды, таблица переменных, «что проверить за минуту», ссылки на `docs/model.md` и `docs/architecture.md` (появятся в этапе 5), раздел «Дополнительно» — список функций вне обязательных (эксперт: «это нужно будет указать в ридми»).

## Отброшенные варианты

| Вариант | Почему нет |
|---|---|
| Отдельный контейнер nginx для фронта | лишняя точка отказа; один образ проще в Compose и Helm (`01_solution.md`) |
| Публиковать порт Postgres | «секьюрно» (эксперт); не нужен для запуска |
| Пароли по умолчанию в `.env.example` | они попадут в публичный репозиторий и на прод |

## Допущения

- [ПРЕДПОЛОЖЕНО] Если `ADMIN_PASSWORD` пуст, приложение не стартует с понятной ошибкой в логе — лучше, чем вход без пароля.
- [ПРОВЕРЕНО] Корневой `README.md` сейчас — описание кита; в `hack-kit/` уже есть `README.md` — при переносе сравнить и не потерять отличия.

## Риски

- `pnpm build` упадёт, пока фронта нет, — поэтому сборка фронта условная.
