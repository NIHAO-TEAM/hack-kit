# health: изменения сценариев

## ADDED

### Requirement: проверка живости методом HEAD

Система SHALL отвечать на `HEAD /healthz` так же, как на `GET`, но без тела.

#### Scenario: HEAD-запрос

- WHEN клиент отправляет `HEAD /healthz`
- THEN ответ `200`, `Content-Type: application/json`, тело пустое, в ответе есть `X-Request-Id`
