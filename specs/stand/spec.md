# stand

Тесты: `backend/internal/platform/webui/webui_test.go` (маршрут SPA), `backend/cmd/api/docs_test.go`, `notfound_test.go`; запуск Compose проверяет CI/стенд.

### Requirement: запуск по README

Система SHALL подниматься с чистой копии репозитория командами из README через Docker Compose.

#### Scenario: первый запуск

- WHEN на машине с Docker выполнены `cp .env.example .env`, заданы пароли и токены, выполнен `docker compose up -d`
- THEN через минуту `GET http://localhost:8080/healthz` отвечает `200`, `GET /api/v1/docs` открывает Swagger, в базе набор организаторов

#### Scenario: перезапуск сохраняет данные

- WHEN выполнены `docker compose down` и снова `docker compose up -d` (без удаления томов)
- THEN данные и настройки те же, что до остановки

#### Scenario: пустой пароль администратора

- WHEN `ADMIN_PASSWORD` в `.env` пуст
- THEN приложение не стартует, в логе названа переменная, которую нужно задать

### Requirement: база не торчит наружу

#### Scenario: порт базы

- WHEN стенд запущен по Compose
- THEN порт PostgreSQL не опубликован на хосте; снаружи доступен только порт приложения

### Requirement: фронт из того же образа

#### Scenario: маршрут SPA

- WHEN браузер открывает `http://localhost:8080/queue`
- THEN отдаётся `index.html` фронта (или страница «фронт не собран», если фронта ещё нет), а не `404`
