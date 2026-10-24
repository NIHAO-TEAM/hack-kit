# Дизайн: проверка README

## Подход

1. `scripts/readme-quickstart.sh`: `awk` вырезает блок между метками; строки `cp .env.example .env` и `docker compose up -d` выполняются как есть; перед `up` скрипт пишет в `.env` случайные значения для пустых обязательных ключей (`POSTGRES_PASSWORD`, `ADMIN_PASSWORD`, `METRICS_TOKEN`, `SOURCE_TOKEN`, демо-пароли) — ровно то, что README просит сделать руками.
2. Ожидание `/healthz` (до 10 мин), затем `curl`-проверки эндпоинтов с сессией (cookie + `X-CSRF-Token` на записи, как в `demo.spec.ts`).
3. Сортировки: для каждого поля `asc` и `desc` — порядок проверяется `jq` по ответу.
4. Перезапуск: `PATCH` веса → `docker compose restart app` → `GET` — значение то же.

## Допущения

- [ПРЕДПОЛОЖЕНО] Жюри запускает на Linux с Docker Compose v2 — как в README.

## Риски

- Секреты в логе: значения не печатаются, только имена ключей.
