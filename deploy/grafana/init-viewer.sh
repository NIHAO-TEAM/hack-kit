#!/bin/sh
# Создаёт учётку жюри с ролью Viewer (провижининг Grafana пользователей не умеет). Идемпотентно.
set -eu
base="http://grafana:3000/grafana"
auth="admin:${GF_SECURITY_ADMIN_PASSWORD}"
i=0
until curl -fsS "$base/api/health" >/dev/null 2>&1; do
  i=$((i + 1)); [ "$i" -gt 60 ] && { echo "grafana не поднялась" >&2; exit 1; }
  sleep 2
done
body=$(printf '{"name":"jury","login":"jury","password":"%s"}' "$JURY_PASSWORD")
code=$(curl -s -o /dev/null -w '%{http_code}' -u "$auth" -H 'Content-Type: application/json' -d "$body" "$base/api/admin/users")
case "$code" in
  200) echo "учётка jury создана" ;;
  412) echo "учётка jury уже есть" ;;
  *) echo "создание jury: HTTP $code" >&2; exit 1 ;;
esac
