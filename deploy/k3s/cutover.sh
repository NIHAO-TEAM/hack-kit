#!/usr/bin/env bash
# Разовое переключение прода с Compose на k3s без смены домена. Запускается на сервере:
#   SHA=<sha> PROD_DOMAIN=<домен> deploy/k3s/cutover.sh
# Все предусловия (sudo, Postgres, дамп) проверяются до остановки Compose; сбой после остановки возвращает стек.
# Порядок: дамп базы Compose → остановка стека Compose (тома остаются, откат — `docker compose up -d`) →
# k3s и Traefik → чарт с replicaCount=0 → восстановление дампа → запуск приложения.
set -euo pipefail

: "${SHA:?нужен SHA}"
: "${PROD_DOMAIN:?нужен PROD_DOMAIN}"
APP_DIR="${APP_DIR:-$HOME/churn}"
SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
NS="${NS:-churn}"
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
export ENV_FILE="$APP_DIR/.env" APP_TAG="${APP_TAG:-$(cat "$APP_DIR/current_tag" 2>/dev/null || echo latest)}"

# Предусловие: sudo без пароля. Без него bootstrap.sh упадёт уже после остановки Compose.
if [ "$(id -u)" -ne 0 ] && ! sudo -n true 2>/dev/null; then
  echo "нужен человек: sudo без пароля ещё не выдан, прод не тронут." >&2
  echo "выполнить на сервере под root: echo '$(id -un) ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/$(id -un)" >&2
  exit 1
fi

dump="$APP_DIR/pre-k3s-$(date +%Y%m%d%H%M%S).sql"
# Контейнеры ищем по меткам Compose (проект churn), а не через `compose ps`: тот требует разобрать
# compose-файл со всеми обязательными переменными .env и падает ещё до проверки контейнеров.
# -a: на повторном запуске после сбоя Postgres Compose уже остановлен, но данные в его томе.
pg="$(docker ps -aq --filter label=com.docker.compose.project=churn --filter label=com.docker.compose.service=postgres | head -n1)"
if [ -n "$pg" ]; then
  echo "Postgres Compose: контейнер $pg; дамп базы в $dump"
  docker start "$pg" >/dev/null
  for _ in $(seq 1 30); do
    docker exec "$pg" pg_isready -U app -d app >/dev/null 2>&1 && break
    sleep 2
  done
  (umask 077; docker exec "$pg" pg_dump -U app --clean --if-exists app > "$dump")
  [ -s "$dump" ] || { echo "дамп пуст, прерываю: стек Compose не тронут" >&2; exit 1; }
  echo "останавливаю стек Compose (тома сохраняются)"
  stopped="$(docker ps -q --filter label=com.docker.compose.project=churn)"
  # Сбой любого шага ниже — вернуть остановленные контейнеры (откат без разбора compose-файла).
  rollback() {
    echo "сбой после остановки Compose: возвращаю контейнеры" >&2
    [ -z "$stopped" ] || echo "$stopped" | xargs -r docker start >/dev/null || true
  }
  trap rollback ERR
  echo "$stopped" | xargs -r docker stop >/dev/null
else
  echo "нужен человек: контейнера Postgres Compose нет, переносить нечего." >&2
  echo "контейнеры на сервере:" >&2
  docker ps -a --format '{{.Names}} {{.Status}} project={{.Label "com.docker.compose.project"}}' >&2 || true
  echo "если прод и так пустой — запустить deploy-k8s без cutover; иначе восстановить дамп $APP_DIR/pre-k3s-*.sql вручную" >&2
  exit 1
fi

bash "$SRC/deploy/k3s/bootstrap.sh"

# Чарт без приложения: поднимается только база, в неё льём дамп.
SKIP_HEALTH=1 SHA="$SHA" PROD_DOMAIN="$PROD_DOMAIN" NS="$NS" bash "$SRC/deploy/k3s/deploy.sh" --set replicaCount=0
kubectl -n "$NS" rollout status statefulset/churn-postgres --timeout=5m
if [ -s "$dump" ]; then
  echo "восстанавливаю дамп"
  kubectl -n "$NS" exec -i churn-postgres-0 -- psql -q -v ON_ERROR_STOP=1 -U app app < "$dump"
fi
SHA="$SHA" PROD_DOMAIN="$PROD_DOMAIN" NS="$NS" bash "$SRC/deploy/k3s/deploy.sh"
trap - ERR
echo "переключено. Дамп: ${dump:-нет}; Compose остановлен, не удалён"
