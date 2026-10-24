# metrics: изменения сценариев

## ADDED

### Requirement: метрики работы с клиентами

`/metrics` SHALL отдавать число задач удержания по статусу, просроченные и ожидаемую потерю клиентов в работе.

#### Scenario: клиент 9 в работе

- WHEN клиента 9 взяли в работу
- THEN `churn_retention_tasks{status="in_work"}` = 1, `churn_retention_expected_loss_in_work_cents` равна ожидаемой потере клиента 9 в API

#### Scenario: срок прошёл

- WHEN срок открытой задачи прошёл
- THEN `churn_retention_tasks_overdue` = 1
