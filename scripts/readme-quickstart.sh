#!/usr/bin/env bash
# Проверка README буквально: шаги блока между метками quickstart выполняются как написаны.
# Секреты — случайные, в лог не печатаются (только имена ключей). Итог — таблица в $GITHUB_STEP_SUMMARY.
# Запуск: из корня клона, чистая копия репозитория. Нужны docker compose, curl, jq, openssl.
set -uo pipefail

BASE="${BASE_URL:-http://localhost:8080}"
SUMMARY="${GITHUB_STEP_SUMMARY:-/dev/null}"
README="${README_FILE:-README.md}"
FAILS=0
JAR="$(mktemp)"
trap 'rm -f "$JAR"' EXIT

echo '| Проверка | Итог |' >>"$SUMMARY"
echo '|---|---|' >>"$SUMMARY"

ok() { echo "ok   $1"; echo "| $1 | ✅ |" >>"$SUMMARY"; }
bad() { echo "FAIL $1"; echo "| $1 | ❌ $2 |" >>"$SUMMARY"; FAILS=$((FAILS + 1)); }
# check <название> <ожидаемый код> <фактический код>
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1" "ждали $2, пришло $3"; fi; }
rand() { openssl rand -hex 16; }

# --- 1. Шаги README как написаны
block="$(awk '/<!-- quickstart:start -->/{f=1;next} /<!-- quickstart:end -->/{f=0} f' "$README")"
cmds="$(printf '%s\n' "$block" | grep -v '^```' | grep -v '^#' | grep -v '^[[:space:]]*$')"
if [ -z "$cmds" ]; then
  bad "блок запуска найден по меткам quickstart" "пусто"
  exit 1
fi
ok "блок запуска найден по меткам quickstart"

# Подсказка «задайте в .env» — делаем руками, как просит README: случайные значения пустым ключам.
fill_env() {
  local key
  for key in POSTGRES_PASSWORD ADMIN_PASSWORD DEMO_MARKETER_PASSWORD DEMO_ANALYST_PASSWORD \
    DEMO_JURY_PASSWORD METRICS_TOKEN SOURCE_TOKEN; do
    if grep -q "^${key}=$" .env; then
      sed -i "s|^${key}=\$|${key}=$(rand)|" .env
      echo "задан ${key}"
    fi
  done
}

while IFS= read -r line; do
  case "$line" in
    "docker compose up"*) fill_env ;;
  esac
  echo "+ $line"
  if ! bash -c "$line"; then
    bad "шаг README: $line" "команда завершилась с ошибкой"
    exit 1
  fi
done <<<"$cmds"
ok "шаги README выполнены"

envval() { grep "^$1=" .env | head -1 | cut -d= -f2-; }

# --- 2. Ожидание /healthz до 10 минут
for _ in $(seq 1 120); do
  [ "$(curl -s -o /dev/null -w '%{http_code}' "$BASE/healthz")" = 200 ] && break
  sleep 5
done
check "GET /healthz" 200 "$(curl -s -o /dev/null -w '%{http_code}' "$BASE/healthz")"
