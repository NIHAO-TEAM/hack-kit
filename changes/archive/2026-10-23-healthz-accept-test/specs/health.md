# health: изменения сценариев

## ADDED

### Requirement: заголовок Accept

Система SHALL отвечать на `GET /healthz` JSON-ом независимо от заголовка `Accept`.

#### Scenario: браузерный Accept

- WHEN клиент отправляет `GET /healthz` с `Accept: text/html`
- THEN ответ `200`, `Content-Type: application/json`, тело `{"status":"ok"}`
