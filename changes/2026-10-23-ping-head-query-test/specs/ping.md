# ping: изменения сценариев

## ADDED

### Requirement: HEAD с параметрами

Система SHALL отвечать на `HEAD /ping` с параметрами запроса так же, как без них.

#### Scenario: параметры в адресе

- WHEN клиент отправляет `HEAD /ping?x=1`
- THEN ответ `200`, тело пустое, `Cache-Control: no-store`
