#!/usr/bin/env bash
# Разовое переключение прода с Compose на k3s без смены домена. Запускается на сервере:
#   SHA=<sha> PROD_DOMAIN=<домен> deploy/k3s/cutover.sh
# Порядок: дамп базы Compose → остановка стека Compose (тома остаются, откат — `docker compose up -d`) →
# k3s и Traefik → чарт с replicaCount=0 → восстановление дампа → запуск приложения.
set -euo pipefail

: "${SHA:?нужен SHA}"
: "${PROD_DOMAIN:?нужен PROD_DOMAIN}"
APP_DIR="${APP_DIR:-$HOME/churn}"
SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
NS="${NS:-churn}"
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
compose() { docker compose -f "$SRC/deploy/docker-compose.prod.yml" --env-file "$APP_DIR/.env" "$@"; }
export ENV_FILE="$APP_DIR/.env" APP_TAG="${APP_TAG:-$(cat "$APP_DIR/current_tag" 2>/dev/null || echo latest)}"

dump="$APP_DIR/pre-k3s-$(date +%Y%m%d%H%M%S).sql"
if [ -n "$(compose ps -q postgres 2>/dev/null)" ]; then
  echo "дамп базы Compose в $dump"
  (umask 077; compose exec -T postgres pg_dump -U app --clean --if-exists app > "$dump")
  echo "останавливаю стек Compose (тома сохраняются)"
  compose stop
else
  echo "стек Compose не запущен, дамп пропущен"
  dump=""
fi

bash "$SRC/deploy/k3s/bootstrap.sh"

# Чарт без приложения: поднимается только база, в неё льём дамп.
SKIP_HEALTH=1 SHA="$SHA" PROD_DOMAIN="$PROD_DOMAIN" NS="$NS" bash "$SRC/deploy/k3s/deploy.sh" --set replicaCount=0
kubectl -n "$NS" rollout status statefulset/churn-postgres --timeout=5m
if [ -n "$dump" ]; then
  echo "восстанавливаю дамп"
  kubectl -n "$NS" exec -i churn-postgres-0 -- psql -q -v ON_ERROR_STOP=1 -U app app < "$dump"
fi
SHA="$SHA" PROD_DOMAIN="$PROD_DOMAIN" NS="$NS" bash "$SRC/deploy/k3s/deploy.sh"
echo "переключено. Дамп: ${dump:-нет}; Compose остановлен, не удалён"
