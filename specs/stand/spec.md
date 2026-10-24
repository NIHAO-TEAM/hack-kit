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

### Requirement: запуск буквально по README

Репозиторий SHALL проверять после каждого выката, что шаги запуска из README на чистом раннере поднимают решение, и все обязательные эндпоинты кейса отвечают как описано.

#### Scenario: чистый раннер

- WHEN выполняются шаги раздела «Запуск» README
- THEN `/healthz` 200, вход работает, обязательные эндпоинты отвечают, без входа — 401, сортировки в обе стороны, настройки переживают перезапуск
- Тест: `deploy-readme-check.yml`

### Requirement: README воспроизводится буквально

Шаги быстрого запуска из README SHALL проходить на чистом стенде, включая приём снимка по сервисному токену, и при провале печатать код и тело ответа без секретов.

#### Scenario: приём снимка по токену

- WHEN по README поднят стенд и вызван `PUT /api/v1/source/snapshot` с токеном из `.env` и снимком из примера
- THEN ответ `200`
- Тест: `Deploy README check`, проверка «PUT /api/v1/source/snapshot по токену»
