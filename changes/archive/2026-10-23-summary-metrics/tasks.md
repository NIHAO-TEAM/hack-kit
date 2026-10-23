# Задачи: сводка и метрики

- [x] 1.1 `GetSummary` — проверка: тест на сиде: уровни 3/2/1/0/4, потеря 387973, под угрозой 690046, потеряно 736834
- [x] 1.2 `GetMetrics`, `GetMetricsAlias` — проверка: `promlint` (тот же набор правил, что у `promtool check metrics`) по ответу; промтул в среде нет; `/metics` = `/metrics`
- [x] 1.3 Инвариант метрики = сводка — проверка: тест
- [x] 2.1 `make check`, `make test-int` зелёные (проверит CI: нет Docker, golangci-lint); сценарии `specs/` проходят
