# Дизайн: прод на Compose + Caddy

## Подход

- **`deploy/docker-compose.prod.yml`:** `postgres` (без `ports`, том), `app` (образ из корневого `Dockerfile`, `env_file: .env`, без `ports`, `expose: 8080`), `caddy:2` (`ports: 80, 443`, тома `caddy_data`, `caddy_config`, `Caddyfile`). Все с `restart: unless-stopped` и healthcheck.
- **`deploy/Caddyfile`:** `{$PROD_DOMAIN} { encode zstd gzip; reverse_proxy app:8080; header { Strict-Transport-Security "max-age=31536000"; X-Content-Type-Options nosniff; Referrer-Policy strict-origin-when-cross-origin; -Server } }`; SSE не буферизуется (`flush_interval -1` для `/api/v1/events`).
- **`deploy/bootstrap.sh`** (идемпотентный, запускается деплоем перед `up`):
  - проверяет `docker` и `docker compose`; нет — ставит через `get.docker.com`, если у пользователя есть `sudo -n`; нет `sudo` — выходит с понятной ошибкой («нужен человек: …»);
  - firewall: `ufw allow 22,80,443/tcp`, `ufw --force enable`, если `ufw` есть и есть `sudo`;
  - кто слушает 80/443 вне наших контейнеров — пишет в лог и выходит с ошибкой (Caddy не получит сертификат);
  - `~/churn/.env` — если нет: генерирует `POSTGRES_PASSWORD`, `ADMIN_PASSWORD`, `DEMO_*_PASSWORD`, `METRICS_TOKEN`, `SOURCE_TOKEN` (`openssl rand -hex 16`), `COOKIE_SECURE=true`, `TRUSTED_PROXY=true`, `SEED_ORGANIZERS=true`, `PROD_DOMAIN`; `chmod 600`; значения **никуда не выводит**.
- **`.github/workflows/deploy.yml`:** `on: workflow_run` (CI на `main`, `conclusion == success`) и `workflow_dispatch`; один job, `concurrency: deploy`: ключ из `secrets.DEPLOY_SSH_KEY` во временный файл, `ssh-keyscan` хоста, по SSH (`vars.PROD_USER@vars.PROD_HOST`): `git -C ~/churn/src fetch && checkout <sha>` (клон публичного репозитория при первом запуске) → `deploy/bootstrap.sh` → `docker compose -f deploy/docker-compose.prod.yml --env-file ~/churn/.env up -d --build` → ожидание `https://${PROD_DOMAIN}/healthz` до 120 с. Не прошло — `docker compose` откатывается на прошлый тег образа (`app:<prev-sha>`), job красный.
- Образ собирается на сервере (репозиторий публичный, реестр не нужен) и тегируется `app:<sha>`; хранятся два последних.

## Отброшенные варианты

| Вариант | Почему нет |
|---|---|
| Секреты прода в GitHub Secrets | их нужно заводить человеку; сгенерированные на сервере не попадают никуда, кроме сервера |
| Сборка образа в Actions + GHCR | нужен токен реестра на сервере; сборка на сервере проще и без новых секретов |
| nginx + certbot | Caddy получает и продлевает сертификат сам |

## Допущения

- [ПРОВЕРЕНО 23.10] Домен указывает на хост, порты 22/80/443 открыты; репозиторий публичный; секрет `DEPLOY_SSH_KEY` и переменные `PROD_*` заведены.
- [НАДЕЮСЬ] У `deploy` есть `sudo` без пароля или уже стоит Docker; кто слушает 80/443 — неизвестно. Если bootstrap упёрся — `[needs-human]` с точными командами, которые выполнить на сервере.
- [ПРЕДПОЛОЖЕНО] Ресурсов VM хватает на Postgres + приложение + Caddy (≈ 1 ГБ).

## Риски

- Лимит Let's Encrypt (5 сертификатов в неделю на домен) — не пересоздавать том `caddy_data`.
- Первый деплой на пустой базе занимает дольше 120 с из-за сборки — таймаут проверки 300 с при первом выкате.
