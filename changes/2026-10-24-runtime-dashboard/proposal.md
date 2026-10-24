Статус: в работе

# Дашборд «Внутри сервиса»: рантайм Go, пересчёт, приём снимка, поток событий

## Зачем

Эксперту aéza интереснее бэкенд и большая нагрузка (ADR-0004, п. 6). Проверено по `deploy/grafana/dashboards/churn.json`: дашборд показывает только бизнес-числа (уровни, потеря, возраст снимка) и «Работу с клиентами», которую ADR-0004 убрал из продукта. Как сервер держит поток 2–3 тыс. событий — горутины, память, паузы GC, длительность пересчёта и этапов снимка, задержка HTTP — на экране не видно, хотя метрики частично уже есть (`churn_snapshot_ingest_stage_seconds`, стандартные `go_*`, `process_*`).

## Что меняется

- Новый дашборд `deploy/grafana/dashboards/runtime.json` «Внутри сервиса»: горутины, куча и выделения/с, паузы GC (p99), CPU процесса; пересчёт — длительность, число, отменённые (если метрика есть); приём снимка по этапам; HTTP — RPS и p95 по маршрутам; поток событий — событий/с.
- Панели, чьих метрик ещё нет на `main` (отмены пересчёта из `recalc-concurrency`, потери шины из `events-bus-fanout`), получают подпись «нет данных до слияния …» — не придумываем ряды.
- Если нет гистограммы длительности HTTP по маршрутам — добавляется в промежуточный слой метрик (`internal/platform/httpx`), с маршрутом-шаблоном, не сырым путём (ограничение кардинальности).
- Из `churn.json` убираются панели «Работа с клиентами», «Открытые задачи по статусу», «Просроченные задачи», «Потеря под работой», «Доля потери под работой» (интерфейса задач нет, ADR-0004, п. 1).
- Правила Prometheus `deploy/prometheus/rules.yml`: запись p95 HTTP и тревога «пересчёт дольше 5 с».
- `docs/load/README.md`: какую панель смотреть при каждом замере.

Вне изменения:
- Бизнес-панели `churn.json`.
- Доступ к Grafana (как настроено).

## Затронутые части

- backend: `internal/platform/httpx` (гистограмма HTTP, если её нет)
- frontend: нет
- контракт `api/openapi.yaml`: нет
- защищённые пути: `deploy/grafana/`, `deploy/prometheus/` (дашборд и правила — по причине выше)
- зависит от: нет

## Связанные решения

- ADR-0004: п. 1 (без CRM), п. 6 (бэкенд и нагрузка).
- Гейт не ослабляется.

## Найдено по ходу

Метрики `/metrics` на `main` (по коду регистраций; сервер в среде не поднимался, свериться на стенде):

- Есть: стандартные `go_*` и `process_*` (`NewGoCollector`, `NewProcessCollector`); `churn_customers`, `churn_expected_loss_cents`, `churn_at_risk_cents`, `churn_lost_cents`, `churn_data_quality_issues`, `churn_snapshot_age_seconds`, `churn_model_version`; `churn_recalc_duration_seconds` и `churn_recalc_stage_seconds` — **gauge последнего пересчёта**, не гистограмма; `churn_snapshot_ingest_stage_seconds` (гистограмма по этапам); `churn_retention_*`; `churn_billing_polls_total`, `churn_billing_poll_errors_total`.
- Нет: гистограммы HTTP (добавлена здесь, `churn_http_request_duration_seconds`), счётчика пересчётов (число пересчётов — `changes(churn_model_version[…])`), отмен пересчёта (`recalc-concurrency`), событий/с и потерь шины (`events-bus-fanout`).
