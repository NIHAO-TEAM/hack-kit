# request-id: изменения сценариев

## ADDED

### Requirement: другие записи UUID

Система SHALL принимать UUID в записи `urn:uuid:…` и в фигурных скобках и возвращать его в каноническом виде.

#### Scenario: urn и скобки

- WHEN клиент отправляет `X-Request-Id: urn:uuid:<uuid>` или `{<uuid>}`
- THEN в ответе `X-Request-Id` равен `<uuid>` в нижнем регистре
