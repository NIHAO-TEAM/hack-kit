# health

### Requirement: проверка живости

Система SHALL отвечать на `GET /healthz` без авторизации, пока процесс API принимает запросы.

#### Scenario: API жив

- WHEN клиент запрашивает `GET /healthz`
- THEN ответ `200`, `Content-Type: application/json`, тело `{"status":"ok"}`
- Тест: `TestHealthz/GET_отвечает_ok`

#### Scenario: неверный метод

- WHEN клиент отправляет `POST /healthz`
- THEN ответ `405`, заголовок `Allow` содержит `GET`
- Тест: `TestHealthz/POST_запрещён`

### Requirement: проверка живости методом HEAD

Система SHALL отвечать на `HEAD /healthz` так же, как на `GET`, но без тела.

#### Scenario: HEAD-запрос

- WHEN клиент отправляет `HEAD /healthz`
- THEN ответ `200`, `Content-Type: application/json`, тело пустое, в ответе есть `X-Request-Id`
- Тест: `TestHealthzHead`

### Requirement: заголовок Accept

Система SHALL отвечать на `GET /healthz` JSON-ом независимо от заголовка `Accept`.

#### Scenario: браузерный Accept

- WHEN клиент отправляет `GET /healthz` с `Accept: text/html`
- THEN ответ `200`, `Content-Type: application/json`, тело `{"status":"ok"}`
- Тест: `TestHealthzAccept/браузерный_Accept`

### Requirement: готовность отдельно от живости

Сервер SHALL отвечать на `GET /readyz` кодом `200` только когда БД доступна и первый пересчёт готов; во время остановки — `503`.

#### Scenario: до первого пересчёта

- WHEN сервер запущен, первый пересчёт не завершён
- THEN `/readyz` отвечает `503`, `/healthz` — `200`
- Тест: `TestReadyHandler/первый_пересчёт_не_готов`, `TestReadyzServed`

### Requirement: плавная остановка

По SIGTERM сервер SHALL перестать принимать новые соединения, дослать идущие запросы, закрыть SSE-потоки и остановить воркеры до выхода.

#### Scenario: выкат под нагрузкой

- WHEN идёт запрос на 300 мс и открыт SSE-поток, приходит SIGTERM
- THEN `/readyz` сразу `503`, запрос получает `200`, SSE закрыт, процесс завершился до `SHUTDOWN_TIMEOUT`
- Тест: `TestServeGracefulShutdown`

### Requirement: готовность на пустой базе

Сервер SHALL считать себя готовым (`/readyz` — `200`), если снимок ещё не загружали: на чистой установке считать нечего.

#### Scenario: чистая установка

- WHEN сервер стартует на пустой базе без снимка
- THEN `/readyz` отвечает `200`
- Тест: `TestReadyOnEmptyDatabase` (`cmd/api/ready_test.go`)

### Requirement: отказ базы не роняет процесс

При недоступной базе сервер SHALL продолжать работать: `/readyz` — `503`, чтение с сессией в кэше (пока жива запись, `SESSION_CACHE_TTL`) — последний готовый результат с заголовком `X-Data-Age`, сессия при промахе кэша — `503` с `Retry-After` в пределах `dbWait` (2 с), без зависаний; запись при остановленной базе — `503` с `Retry-After`, при зависшей (пауза процесса) — ждёт её возврата; после возврата базы события потока применены без потерь.

#### Scenario: база пропала на 10 с под залпом

- WHEN идёт залп 10 000 событий и PostgreSQL недоступен 10 с
- THEN процесс не перезапускался, ответов `500` нет, после возврата все события применены, согласованность 0 центов
- Тест: режим `outage` в `deploy-burst.yml` (`docs/load/outage.sh`); `TestIsDBUnavailable`, `TestMiddlewareDBDown`, `TestRetryCommitWaitsForDB`, `TestDataAge`
