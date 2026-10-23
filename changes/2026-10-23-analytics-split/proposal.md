Статус: в работе

# Разнести обработчики `analytics` по файлам (без изменения поведения)

## Зачем

Блокирующее изменение этапа 4 (`!analytics-split`, `docs/plan.md`). Все обработчики модуля `analytics` лежат в одном `http.go`; `customers-api`, `summary-metrics` и `live-updates` иначе правят один файл параллельно и ловят конфликты.

## Что меняется

- Методы из `internal/analytics/http.go` переезжают по файлам: `customers.go` (`ListCustomers`, `GetCustomer`), `summary.go` (`GetSummary`), `chart.go` (`GetChurnChart`), `events.go` (`GetEvents`), `metrics.go` (`GetMetrics`, `GetMetricsAlias`). В `http.go` остаются тип `AnalyticsAPI` и `New`.
- Поведение не меняется.

Вне изменения:
- Реализация обработчиков — следующие изменения этапа 4.

## Затронутые части

- backend: `internal/analytics`
- frontend: нет
- контракт `api/openapi.yaml`: нет
- защищённые пути: нет
- зависит от: `recalc-engine`

## Связанные решения

- ADR: 0001
- Макет: не нужен

## Найдено по ходу

