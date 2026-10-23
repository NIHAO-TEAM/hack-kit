# request-id: изменения сценариев

## ADDED

### Requirement: регистр имени заголовка

Система SHALL принимать заголовок запроса `X-Request-Id` с именем в любом регистре.

#### Scenario: имя в нижнем регистре

- WHEN клиент отправляет `x-request-id: <uuid>`
- THEN в ответе `X-Request-Id` равен `<uuid>`
