# Дизайн: метрики задач

## Подход

1. Сборщик `prometheus.Collector` в `internal/retention`: при сборе — один агрегирующий запрос (`GROUP BY status`, `count(*) FILTER (WHERE due_at < now())`, закрытые по `outcome`). Регистрация рядом с текущими метриками приложения.
2. `churn_retention_expected_loss_in_work_cents` — сумма `expectedLossCents` текущего результата модели по `customer_id` открытых задач, у которых клиент есть в снимке (та же сверка id + email, что `customerMissing`).
3. `closed_total` — gauge по таблице (не счётчик в памяти), чтобы переживать рестарт; имя без `_total`, если `TestMetricsLint` (promlint) требует counter для `_total`: `churn_retention_tasks_closed{outcome}`.
4. Дашборд: новая строка панелей в `churn.json`, источник — тот же Prometheus.

## Допущения

- [ПРЕДПОЛОЖЕНО] Ошибка БД при сборе — метрики задач пропускаются, остальные `/metrics` отдаются (не 500).

## Риски

- `TestMetricsMatchSummary` и `TestMetricsLint` должны остаться зелёными; новые метрики — в их проверку.
