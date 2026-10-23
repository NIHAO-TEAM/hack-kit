#!/usr/bin/env bash
# Оставляет на проде один вход на 80/443 — Traefik из k3s: стек Compose (Caddy, приложение, база)
# останавливается, тома остаются (откат — `docker compose up -d`). Идемпотентно. Запускается на сервере.
set -euo pipefail

APP_DIR="${APP_DIR:-$HOME/churn}"
SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
export KUBECONFIG="${KUBECONFIG:-/etc/rancher/k3s/k3s.yaml}"
export ENV_FILE="$APP_DIR/.env" APP_TAG="${APP_TAG:-$(cat "$APP_DIR/current_tag" 2>/dev/null || echo latest)}"

if ! kubectl -n churn get deploy,statefulset >/dev/null 2>&1; then
  echo "нужен человек: релиз churn в k3s не найден, Compose не трогаю (единственный вход)." >&2
  exit 1
fi

if [ -n "$(docker ps -q --filter publish=80 --filter publish=443 2>/dev/null)" ]; then
  echo "порты 80/443 держит Docker: останавливаю стек Compose"
  docker compose -f "$SRC/deploy/docker-compose.prod.yml" --env-file "$ENV_FILE" stop
else
  echo "Compose уже остановлен"
fi
