Статус: в работе

# `GET /analytics/summary` и `/metrics` для Prometheus

## Зачем

Кейс: «Денежная оценка… сколько бизнес может потерять»; «Endpoint `/metics`… как минимум количество клиентов по уровням риска и денежную оценку»; приёмка: «`/metrics` доступен сборщику, содержит измеренные данные и согласован с главными показателями решения» (`docs/case.md`). Эксперт: метрики собирают в Prometheus.

## Что меняется

- `GET /api/v1/analytics/summary` — клиенты по уровням (`high`, `medium`, `low`, `unscored`, `lost`), ожидаемая потеря, под угрозой, уже потеряно, `asOf`, версия, счётчики проверок качества.
- `GET /metrics` и `GET /metics` — один обработчик в формате Prometheus: `churn_customers{level}`, `churn_expected_loss_cents`, `churn_at_risk_cents`, `churn_lost_cents`, `churn_data_quality_issues{check}`, `churn_recalc_duration_seconds`, `churn_snapshot_age_seconds`, `churn_model_version` и стандартные метрики процесса; единицы — в `HELP`.
- Сводка и метрики берутся из одного результата пересчёта.

Вне изменения:
- Prometheus и Grafana на проде — этапы 6–7. Доступ к `/metrics` уже сделан в `auth-sessions`.

## Затронутые части

- backend: `internal/analytics/summary.go`, `internal/analytics/metrics.go`
- frontend: нет
- контракт `api/openapi.yaml`: нет
- защищённые пути: нет
- зависит от: `analytics-split`

## Связанные решения

- ADR: 0001; инвариант «Σ `/metrics` = `/analytics/summary`» (`docs/decision/03_verdict.md`)
- Макет: не нужен

## Найдено по ходу

