# api-contract: изменения сценариев

## ADDED

### Requirement: спека и документация открыты

Система SHALL отдавать спецификацию API и Swagger UI без входа; данных клиентов в них нет.

#### Scenario: Swagger без входа

- WHEN клиент без сессии запрашивает `GET /api/v1/docs`
- THEN ответ `200`, HTML Swagger UI, все ресурсы UI загружаются с того же сервера

#### Scenario: спека в YAML и JSON

- WHEN клиент запрашивает `GET /api/v1/openapi.yml` или `GET /api/v1/openapi.json`
- THEN ответ `200`, тело — спецификация, совпадающая с `api/openapi.yaml`

### Requirement: формат ошибок

Система SHALL отвечать на ошибки `application/problem+json` с полями `type`, `title`, `status`, `detail`, `correlation_id`.

#### Scenario: путь ещё не реализован

- WHEN клиент вызывает путь из спеки, модуль которого не реализован
- THEN ответ `501`, `Content-Type: application/problem+json`, `correlation_id` равен заголовку `X-Request-Id` ответа

### Requirement: служебные пути сохранены

Система SHALL отвечать на `GET /healthz`, `GET /version`, `GET /ping` так же, как до изменения.

#### Scenario: проверка живости

- WHEN клиент запрашивает `GET /healthz`
- THEN ответ `200`, тело `{"status":"ok"}`

### Requirement: схема данных организаторов

Система SHALL хранить аккаунты, услуги и транзакции с полями и ограничениями `cases/analizator/001_schema.up.sql`.

#### Scenario: данные организаторов загружаются в схему

- WHEN `cases/analizator/002_seed.sql` применяется к базе после миграций
- THEN загружено 10 аккаунтов, 30 услуг, 100 транзакций без ошибок ограничений
