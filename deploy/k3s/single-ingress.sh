#!/usr/bin/env bash
# Оставляет на проде один вход на 80/443 — Traefik из k3s. Из стека Compose останавливаются Caddy,
# приложение и база (их заменил k3s); Keycloak, Grafana и Prometheus продолжают работать в Compose,
# порты у них только на COMPANIONS_BIND (адрес docker0), снаружи закрыты. Тома остаются
# (откат — `docker compose up -d`). Идемпотентно. Запускается на сервере.
set -euo pipefail

APP_DIR="${APP_DIR:-$HOME/churn}"
SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
export KUBECONFIG="${KUBECONFIG:-/etc/rancher/k3s/k3s.yaml}"
export ENV_FILE="$APP_DIR/.env" APP_TAG="${APP_TAG:-$(cat "$APP_DIR/current_tag" 2>/dev/null || echo latest)}"
COMPOSE=(docker compose -f "$SRC/deploy/docker-compose.prod.yml" --env-file "$ENV_FILE")

if ! kubectl -n churn get deploy,statefulset >/dev/null 2>&1; then
  echo "нужен человек: релиз churn в k3s не найден, Compose не трогаю (единственный вход)." >&2
  exit 1
fi

# Спутники ходят в k3s и обратно через адрес docker0 и публичный домен; значения — в .env, без значений в логе.
set_env() { # ключ, значение: дописать, если ключа ещё нет
  grep -q "^$1=" "$ENV_FILE" || { printf '%s=%s\n' "$1" "$2" >> "$ENV_FILE"; echo "$1 добавлен в .env"; }
}
domain="$(grep -E '^PROD_DOMAIN=' "$ENV_FILE" | cut -d= -f2-)"
set_env COMPANIONS_BIND 172.17.0.1
set_env METRICS_TARGET "$domain:443"
set_env METRICS_SCHEME https

if [ -n "$(docker ps -q --filter publish=80 --filter publish=443 2>/dev/null)" ]; then
  echo "порты 80/443 держит Docker: останавливаю Caddy, приложение и базу Compose"
  "${COMPOSE[@]}" stop caddy app postgres
else
  echo "Caddy Compose уже остановлен"
fi

# --no-deps: приложение и база Compose не поднимаются заново как зависимости.
echo "поднимаю спутников: keycloak (профиль sso, если включён), prometheus, grafana"
"${COMPOSE[@]}" up -d --no-deps keycloak prometheus grafana
"${COMPOSE[@]}" up --no-deps grafana-init >/dev/null 2>&1 || true

# Keycloak стартует 1–2 минуты: ждём discovery до 180 с. Не ответил — лог в job и красный шаг
# (прод не откатываем: приложение работает, вход по паролю — запасной).
bind="$(grep -E '^COMPANIONS_BIND=' "$ENV_FILE" | cut -d= -f2-)"
url="http://${bind:-172.17.0.1}:18080/sso/realms/churn/.well-known/openid-configuration"
for _ in $(seq 1 36); do
  if [ "$(curl -s -o /dev/null --max-time 5 -w '%{http_code}' "$url")" = 200 ]; then
    echo "Keycloak: discovery отвечает 200"; exit 0
  fi
  sleep 5
done
echo "Keycloak не ответил за 180 с; последние строки лога (без секретов):" >&2
"${COMPOSE[@]}" ps keycloak >&2 || true
"${COMPOSE[@]}" logs --tail 60 keycloak 2>&1 | grep -v -i -E 'password|secret|token|totp' >&2 || true
exit 1
