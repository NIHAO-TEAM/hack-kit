# ping: изменения сценариев

## ADDED

### Requirement: параметры запроса

Система SHALL отвечать на `GET /ping` с любыми параметрами запроса так же, как без них.

#### Scenario: параметры в адресе

- WHEN клиент отправляет `GET /ping?x=1`
- THEN ответ `200`, тело `{"ping":"pong"}`, `Cache-Control: no-store`
