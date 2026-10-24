# api-contract

### Requirement: спека и документация открыты

Система SHALL отдавать спецификацию API и Swagger UI без входа; данных клиентов в них нет.

#### Scenario: Swagger без входа

- WHEN клиент без сессии запрашивает `GET /api/v1/docs`
- THEN ответ `200`, HTML Swagger UI, все ресурсы UI загружаются с того же сервера
- Тест: `TestDocsOpenWithoutLogin` (`backend/cmd/api/docs_test.go`)

#### Scenario: спека в YAML и JSON

- WHEN клиент запрашивает `GET /api/v1/openapi.yml` или `GET /api/v1/openapi.json`
- THEN ответ `200`, тело — спецификация, совпадающая с `api/openapi.yaml`
- Тест: `TestOpenAPIYamlIsContractFile`, `TestOpenAPIJson`

### Requirement: формат ошибок

Система SHALL отвечать на ошибки `application/problem+json` с полями `type`, `title`, `status`, `detail`, `correlation_id`.

#### Scenario: путь ещё не реализован

- WHEN клиент вызывает путь из спеки, модуль которого не реализован
- THEN ответ `501`, `Content-Type: application/problem+json`, `correlation_id` равен заголовку `X-Request-Id` ответа
- Тест: `TestEveryUnimplementedPathAnswers501`, `TestWriteProblemFormat`

### Requirement: служебные пути сохранены

Система SHALL отвечать на `GET /healthz`, `GET /version`, `GET /ping` так же, как до изменения.

#### Scenario: проверка живости

- WHEN клиент запрашивает `GET /healthz`
- THEN ответ `200`, тело `{"status":"ok"}`
- Тест: `TestHealthz` и остальные `backend/cmd/api/*_test.go` (без изменений)

### Requirement: схема данных организаторов

Система SHALL хранить аккаунты, услуги и транзакции с полями и ограничениями `cases/analizator/001_schema.up.sql`.

#### Scenario: данные организаторов загружаются в схему

- WHEN `cases/analizator/002_seed.sql` применяется к базе после миграций
- THEN загружено 10 аккаунтов, 30 услуг, 100 транзакций без ошибок ограничений
- Тест: `TestOrganizerSeedLoadsIntoSchema`, `TestMigrationsUpDownUp` (`backend/internal/platform/db`)

### Requirement: ответы ошибок описаны в контракте

Каждый код ошибки, который сервер отдаёт на путь, SHALL быть описан у этого пути в `api/openapi.yaml`.

#### Scenario: перебор пароля

- WHEN шестая неверная попытка входа подряд
- THEN 429, ответ соответствует описанию в контракте
- Тест: `TestErrorResponsesMatchContract/429` (`backend/internal/apitest/contract_errors_test.go`)

#### Scenario: cookie без CSRF

- WHEN `PATCH /api/v1/model/settings` по cookie без `X-CSRF-Token`
- THEN 403, ответ соответствует описанию в контракте
- Тест: `TestErrorResponsesMatchContract/403` (`backend/internal/apitest/contract_errors_test.go`)

#### Scenario: слишком большой снимок

- WHEN тело `PUT /api/v1/source/snapshot` больше `SNAPSHOT_MAX_BYTES`
- THEN 400 `application/problem+json`, ответ соответствует описанию в контракте
- Тест: `TestErrorResponsesMatchContract/слишком_большой_снимок`

### Requirement: кривой ввод не даёт 5xx

Сервер SHALL отвечать на некорректные запросы к любой операции контракта кодом `4xx` в `application/problem+json` с `correlation_id` и никогда — `5xx` (кроме `501`, описанного в контракте).

#### Scenario: строка вместо числа

- WHEN в параметр `limit` списка клиентов передана строка, а в тело `PATCH /model/settings` — отрицательный вес
- THEN ответ `400` или `422` в `problem+json`, не `500`
- Тест: `TestNegativeSweep`, `FuzzAPI` (`backend/internal/apitest/negative_sweep_test.go`)

#### Scenario: неизвестный путь под /api/

- WHEN запрос к несуществующему пути или методу под `/api/`
- THEN `404` в `application/problem+json` с `correlation_id`
- Тест: `TestNegativeSweep`

### Requirement: перегрузка даёт быстрый 503

Сервер SHALL отвечать `503` с `Retry-After` на запросы сверх предела одновременных запросов своего класса не позже чем через 150 мс, обслуживая принятые запросы.

#### Scenario: читателей в 10 раз больше предела

- WHEN одновременно приходят запросы чтения в 10 раз сверх предела
- THEN лишние получают `503` с `Retry-After` быстрее 150 мс, принятые — `200`, `/readyz` отвечает
- Тест: `TestLimitShedsFastAndServesAccepted`, `TestLimitClasses` (`internal/platform/httpx/limit_test.go`)
