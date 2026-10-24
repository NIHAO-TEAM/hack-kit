# grafana

### Requirement: мониторинг по ссылке

#### Scenario: жюри

- WHEN жюри входит в Grafana учёткой viewer
- THEN дашборд показывает те же уровни и потерю, что панель

### Requirement: дашборд «Внутри сервиса»

Grafana SHALL показывать дашборд «Внутри сервиса» (`runtime.json`) с рантаймом Go, пересчётом, этапами приёма снимка, HTTP по маршрутам и потоком событий; каждая панель ссылается только на метрики, которые отдаёт сервер, а панель про метрику, которой ещё нет, помечена «нет данных до слияния».

#### Scenario: нагрузочный замер

- WHEN идёт поток событий эмулятора
- THEN на дашборде видны событий/с, горутины, куча, паузы GC и длительность пересчёта
- Тест: `TestRuntimeDashboardUsesKnownMetrics` (`backend/cmd/api/dashboards_test.go`)

#### Scenario: метка маршрута HTTP

- WHEN приходят запросы к `/api/v1/customers/1` и `/api/v1/customers/2`
- THEN в `churn_http_request_duration_seconds` один ряд с шаблоном маршрута, а неизвестный путь идёт как `unmatched`
- Тест: `TestInstrumentUsesRoutePattern`

#### Scenario: тревога пересчёта

- WHEN последний пересчёт длится дольше 5 с две минуты подряд
- THEN срабатывает `RecalcSlow`
- Тест: `TestPrometheusRulesParse`

### Requirement: панели «Работа с клиентами» убраны (ADR-0004, п. 1)

Дашборд `churn.json` SHALL NOT показывать панели задач (`churn_retention_*`).

#### Scenario: бизнес-дашборд

- WHEN открыт `churn.json`
- THEN в нём нет строки «Работа с клиентами» и запросов к `churn_retention_*`
- Тест: `TestChurnDashboardHasNoTaskPanels`
