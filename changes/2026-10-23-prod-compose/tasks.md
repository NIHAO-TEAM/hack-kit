# Задачи: прод на Compose + Caddy

Каждая задача проверяема: в конце строки — чем проверить.

## 1. Контракт и данные

- нет

## 2. Логика

- [x] 2.1 `deploy/docker-compose.prod.yml` и `deploy/Caddyfile` — проверка: `docker compose -f deploy/docker-compose.prod.yml config` без ошибок; у `postgres` и `app` нет `ports`
- [ ] 2.2 `deploy/bootstrap.sh` (Docker, firewall, проверка 80/443, генерация `.env` без вывода значений) — проверка: `shellcheck`; повторный запуск не меняет `.env`
- [ ] 2.3 `.github/workflows/deploy.yml` (после зелёного CI на `main`, SSH по `DEPLOY_SSH_KEY`, сборка на сервере, проверка `/healthz`, откат) — проверка: `actionlint`; в логе job нет значений секретов
- [ ] 2.4 Первый выкат вручную (`workflow_dispatch` с ветки PR или после слияния) — проверка: `curl -fsS https://nihao-churn.duckdns.org/healthz` → `{"status":"ok"}`; `curl -I http://…` → редирект на https
- [ ] 2.5 Снаружи открыты только 22/80/443 — проверка: с другой машины порты 5432 и 8080 закрыты

## 3. Экран

- нет

## 4. Готовность

- [ ] 4.1 `make check` зелёный (код не затронут)
- [ ] 4.2 Сценарии из `specs/` проходят на проде
- [ ] 4.3 В «Найдено по ходу»: как человеку забрать пароли учёток с сервера (команда), без самих значений
