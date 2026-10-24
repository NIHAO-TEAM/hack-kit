# metrics

### Requirement: сводка

Система SHALL отдавать сводку риска из последнего пересчёта.

#### Scenario: данные организаторов

- WHEN запрошен `GET /api/v1/analytics/summary`
- THEN 3 клиента высокого риска, 2 среднего, 1 низкого, 4 потеряны; ожидаемая потеря 3 599,07 €
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

### Requirement: метрики работы с клиентами

`/metrics` SHALL отдавать число задач удержания по статусу, просроченные, закрытые по итогу и ожидаемую потерю клиентов в работе. Метки сотрудников и email в метриках нет.

#### Scenario: клиент 9 в работе

- WHEN клиента 9 взяли в работу
- THEN `churn_retention_tasks{status="in_work"}` = 1, `churn_retention_expected_loss_in_work_cents` равна ожидаемой потере клиента 9 в API
- Тест: `TestTaskMetrics`

#### Scenario: срок прошёл

- WHEN срок открытой задачи прошёл
- THEN `churn_retention_tasks_overdue` = 1
- Тест: `TestTaskMetrics`

#### Scenario: клиента нет в снимке

- WHEN клиент открытой задачи пропал из снимка или у него другой email
- THEN его потеря в `churn_retention_expected_loss_in_work_cents` не считается, а сама задача остаётся в счётчиках
- Тест: `TestTaskMetrics`

### Requirement: работа с клиентами в сводке

Ответ сводки SHALL содержать долю ожидаемой потери в работе и клиентов высокого риска без работы — те же числа, что `/metrics`. Блок `retention` в ответе необязателен: при ошибке чтения задач сводка отдаётся без него. Экран «Сводка» блок не показывает (ADR-0004, `retention-crm-cut`): данные остаются для метрик.

#### Scenario: клиент 9 в работе

- WHEN на наборе организаторов в работе только клиент 9
- THEN в ответе сводки данные по клиенту 9; `churn_retention_expected_loss_in_work_cents` = 318910
- Тест: `TestSummaryRetentionMatchesMetrics`, `TestSummaryRetention`

#### Scenario: ошибка чтения задач

- WHEN база задач недоступна
- THEN `GET /api/v1/analytics/summary` отвечает 200 без поля `retention`
- Тест: `TestSummaryRetentionTasksError`
