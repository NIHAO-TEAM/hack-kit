# metrics: изменения сценариев

## ADDED

### Requirement: сводка

#### Scenario: данные организаторов

- WHEN запрошен `GET /api/v1/analytics/summary`
- THEN 3 клиента высокого риска, 2 среднего, 1 низкого, 4 потеряны; ожидаемая потеря 3 879,73 €

### Requirement: метрики для Prometheus

#### Scenario: сборщик

- WHEN сборщик с токеном запрашивает `GET /metrics`
- THEN ответ в формате Prometheus с числом клиентов по уровням и ожидаемой потерей, равными сводке

#### Scenario: путь из текста кейса

- WHEN запрошен `GET /metics`
- THEN ответ тот же, что у `/metrics`
