#!/usr/bin/env bash
# Подготовка сервера к выкату. Идемпотентный: повторный запуск не меняет ни .env, ни firewall.
# Запускается деплоем на сервере перед `docker compose up`. Значения секретов не выводит.
set -euo pipefail

APP_DIR="${APP_DIR:-$HOME/churn}"
ENV_FILE="$APP_DIR/.env"
: "${PROD_DOMAIN:?нужен человек: не задан PROD_DOMAIN}"

sudo_cmd() {
  if [ "$(id -u)" -eq 0 ]; then "$@"; else sudo -n "$@"; fi
}
can_sudo() { [ "$(id -u)" -eq 0 ] || sudo -n true 2>/dev/null; }

# 1. Docker и compose.
if ! docker compose version >/dev/null 2>&1; then
  if ! can_sudo; then
    echo "нужен человек: на сервере нет Docker Compose и нет sudo без пароля." >&2
    echo "выполнить на сервере под root: curl -fsSL https://get.docker.com | sh && usermod -aG docker $(id -un)" >&2
    exit 1
  fi
  echo "ставлю Docker"
  curl -fsSL https://get.docker.com | sudo_cmd sh
  sudo_cmd usermod -aG docker "$(id -un)" || true
fi
if ! docker info >/dev/null 2>&1; then
  echo "нужен человек: пользователь $(id -un) не может работать с Docker." >&2
  echo "выполнить на сервере под root: usermod -aG docker $(id -un)" >&2
  exit 1
fi

# 2. Firewall: открыты только 22, 80, 443.
if command -v ufw >/dev/null 2>&1 && can_sudo; then
  sudo_cmd ufw allow 22/tcp >/dev/null
  sudo_cmd ufw allow 80/tcp >/dev/null
  sudo_cmd ufw allow 443/tcp >/dev/null
  sudo_cmd ufw --force enable >/dev/null
  echo "ufw: открыты 22, 80, 443"
else
  echo "ufw недоступен или нет sudo: firewall не настроен, проверьте порты вручную"
fi

# 3. 80/443 не должны быть заняты чужим процессом (иначе Caddy не получит сертификат).
ours="$(docker ps --filter publish=80 --filter publish=443 --format '{{.Names}}' 2>/dev/null || true)"
if [ -z "$ours" ] && command -v ss >/dev/null 2>&1; then
  busy="$(ss -ltnH 'sport = :80 or sport = :443' 2>/dev/null || true)"
  if [ -n "$busy" ]; then
    echo "нужен человек: порт 80 или 443 занят не нашими контейнерами:" >&2
    echo "$busy" >&2
    echo "освободить на сервере (например, systemctl disable --now nginx apache2) и перезапустить деплой" >&2
    exit 1
  fi
fi

# 4. .env прода: создаётся один раз, значения нигде не печатаются.
mkdir -p "$APP_DIR"
if [ -f "$ENV_FILE" ]; then
  echo ".env уже есть, не трогаю"
else
  gen() { openssl rand -hex 16; }
  umask 077
  cat > "$ENV_FILE" <<ENV
POSTGRES_PASSWORD=$(gen)
ADMIN_LOGIN=admin
ADMIN_PASSWORD=$(gen)
DEMO_MARKETER_PASSWORD=$(gen)
DEMO_ANALYST_PASSWORD=$(gen)
DEMO_JURY_PASSWORD=$(gen)
METRICS_TOKEN=$(gen)
SOURCE_TOKEN=$(gen)
SEED_ORGANIZERS=true
COOKIE_SECURE=true
TRUSTED_PROXY=true
PROD_DOMAIN=$PROD_DOMAIN
ENV
  chmod 600 "$ENV_FILE"
  echo ".env создан (права 600)"
fi
