# metrics

### Requirement: сводка

Система SHALL отдавать сводку риска из последнего пересчёта.

#### Scenario: данные организаторов

- WHEN запрошен `GET /api/v1/analytics/summary`
- THEN 3 клиента высокого риска, 2 среднего, 1 низкого, 4 потеряны; ожидаемая потеря 3 879,73 €
- Тест: `TestSummary` (`internal/analytics/summary_metrics_test.go`), `TestSummaryAndMetricsOnSeed` (`internal/app/summary_integration_test.go`)

#### Scenario: первый пересчёт не закончен

- WHEN запрошена сводка до первого пересчёта
- THEN ответ `503`
- Тест: `TestSummaryBeforeFirstRecalc`

### Requirement: метрики для Prometheus

Система SHALL отдавать метрики в формате Prometheus из того же результата пересчёта, что и сводка.

#### Scenario: сборщик

- WHEN сборщик с токеном запрашивает `GET /metrics`
- THEN ответ в формате Prometheus с числом клиентов по уровням и ожидаемой потерей, равными сводке
- Тест: `TestMetricsMatchSummary`, `TestMetricsLint`, `TestMetricsEndpointWithToken`

#### Scenario: путь из текста кейса

- WHEN запрошен `GET /metics`
- THEN ответ тот же, что у `/metrics`
- Тест: `TestMetricsAliasSameAsMetrics`

### Requirement: сбор на проде

Система SHALL собирать метрики Prometheus на проде, не открывая его наружу.

#### Scenario: прод

- WHEN Prometheus на проде опрашивает приложение
- THEN цель в состоянии UP, порт Prometheus не открыт снаружи
- Тест: нет автотеста; конфиг — `deploy/prometheus/`, target UP проверяется при выкате
