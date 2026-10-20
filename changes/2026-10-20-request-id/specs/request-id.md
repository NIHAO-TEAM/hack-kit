# request-id: изменения сценариев

## ADDED

### Requirement: идентификатор запроса

Система SHALL возвращать в каждом ответе заголовок `X-Request-Id` с UUID.

#### Scenario: заголовка в запросе нет

- WHEN клиент запрашивает `GET /healthz` без `X-Request-Id`
- THEN в ответе есть `X-Request-Id` со значением, разбираемым как UUID

#### Scenario: клиент прислал UUID

- WHEN клиент запрашивает `GET /healthz` с `X-Request-Id`, равным корректному UUID
- THEN в ответе `X-Request-Id` равен присланному значению

#### Scenario: клиент прислал мусор

- WHEN клиент запрашивает `GET /healthz` с `X-Request-Id: not-a-uuid`
- THEN в ответе `X-Request-Id` — новый UUID, не равный присланной строке

#### Scenario: ответ с ошибкой

- WHEN клиент отправляет `POST /healthz`
- THEN ответ `405` и в нём есть `X-Request-Id`
