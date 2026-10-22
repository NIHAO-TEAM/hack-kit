# ping: изменения сценариев

## ADDED

### Requirement: проверка связи методом HEAD

Система SHALL отвечать на `HEAD /ping` так же, как на `GET`, но без тела.

#### Scenario: HEAD-запрос

- WHEN клиент отправляет `HEAD /ping`
- THEN ответ `200`, тело пустое, `Cache-Control: no-store`, в ответе есть `X-Request-Id`
