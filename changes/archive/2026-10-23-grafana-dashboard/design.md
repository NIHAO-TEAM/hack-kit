# Дизайн: Grafana на проде: дашборд за входом

## Подход

- `GF_SERVER_ROOT_URL`/`SERVE_FROM_SUB_PATH` для `/grafana`; источник данных Prometheus — провижининг.
- Grafana 11.3.0, сервис `grafana` в `deploy/docker-compose.prod.yml`; наружу только через Caddy `handle /grafana*`. Анонимный доступ и регистрация выключены, пароль админа — `GRAFANA_ADMIN_PASSWORD` из `.env`.
- Учётка жюри (`jury`, роль по умолчанию Viewer) создаётся разовым сервисом `grafana-init` через API (`deploy/grafana/init-viewer.sh`), пароль — `GRAFANA_JURY_PASSWORD`. Повторный запуск безопасен (412 = уже есть).
- Пароли дописывает `bootstrap.sh` в уже существующий `.env`, если ключей нет, — иначе выкат на текущем проде упал бы на `:?`.

## Решения по ходу

- [ПРЕДПОЛОЖЕНО] В приложении нет HTTP-метрик запросов (в `/metrics` только `churn_*`). Панель «HTTP» показывает `up` и `scrape_duration_seconds` по job `churn`; добавлять метрики в бэкенд — вне объёма («backend: нет»).
- [ПРЕДПОЛОЖЕНО] Только Compose-прод, как в `prometheus-prod`; k3s — вне изменения.
- [ПРЕДПОЛОЖЕНО] Docker, `promtool` и прод в среде нет: JSON дашборда, YAML и shell проверены синтаксически, работу проверит выкат.
