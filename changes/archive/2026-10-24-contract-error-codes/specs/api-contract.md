# api-contract: изменения сценариев

## ADDED

### Requirement: ответы ошибок описаны в контракте

Каждый код ошибки, который сервер отдаёт на путь, SHALL быть описан у этого пути в `api/openapi.yaml`.

#### Scenario: перебор пароля

- WHEN шестая неверная попытка входа подряд
- THEN 429, ответ соответствует описанию в контракте

#### Scenario: cookie без CSRF

- WHEN `PATCH /api/v1/model/settings` по cookie без `X-CSRF-Token`
- THEN 403, ответ соответствует описанию в контракте
