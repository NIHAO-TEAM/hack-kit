# version: изменения сценариев

## ADDED

### Requirement: версия методом HEAD

Система SHALL отвечать на `HEAD /version` так же, как на `GET`, но без тела.

#### Scenario: HEAD-запрос

- WHEN клиент отправляет `HEAD /version`
- THEN ответ `200`, `Content-Type: application/json`, тело пустое, в ответе есть `X-Request-Id`
