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
BODY="$(mktemp)" # тело последнего ответа code(): при провале проверки печатается в лог
trap 'rm -f "$JAR" "$BODY"' EXIT

echo '| Проверка | Итог |' >>"$SUMMARY"
echo '|---|---|' >>"$SUMMARY"

ok() { echo "ok   $1"; echo "| $1 | ✅ |" >>"$SUMMARY"; }
bad() { echo "FAIL $1"; echo "| $1 | ❌ $2 |" >>"$SUMMARY"; FAILS=$((FAILS + 1)); }
# Начало тела последнего ответа без значений секретов из .env (токены и пароли).
body_head() {
  local out key val
  out="$(head -c 300 "$BODY" 2>/dev/null | tr '\n' ' ')"
  for key in POSTGRES_PASSWORD ADMIN_PASSWORD DEMO_MARKETER_PASSWORD DEMO_ANALYST_PASSWORD \
    DEMO_JURY_PASSWORD METRICS_TOKEN SOURCE_TOKEN; do
    val="$(grep "^${key}=" .env 2>/dev/null | head -1 | cut -d= -f2-)"
    [ -n "$val" ] && out="${out//"$val"/***}"
  done
  printf '%s' "$out"
}
# check <название> <ожидаемый код> <фактический код>
check() {
  if [ "$2" = "$3" ]; then
    ok "$1"
  else
    bad "$1" "ждали $2, пришло $3"
    echo "     HTTP $3, тело: $(body_head)"
  fi
}
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

# --- 3. Обязательные эндпоинты
code() { curl -s -o "$BODY" -w '%{http_code}' "$@"; }
check "GET / (веб-клиент)" 200 "$(code "$BASE/")"
check "GET /api/v1/docs (Swagger)" 200 "$(code "$BASE/api/v1/docs")"

# без входа — 401 на путях с данными
for p in /api/v1/auth/me /api/v1/customers /api/v1/customers/1 /api/v1/analytics/summary /api/v1/model/settings; do
  check "без входа 401: GET $p" 401 "$(code "$BASE$p")"
done
check "без входа 401: PUT /api/v1/source/snapshot" 401 \
  "$(code -X PUT -H 'Content-Type: application/json' -d '{}' "$BASE/api/v1/source/snapshot")"
check "без входа 401: GET /metrics" 401 "$(code "$BASE/metrics")"

# вход jury (демо-пароль из .env), cookie + CSRF
JURY_PW="$(envval DEMO_JURY_PASSWORD)"
login_body="$(jq -n --arg p "$JURY_PW" '{login:"jury",password:$p}')"
login_resp="$(curl -s -c "$JAR" -w '\n%{http_code}' -H 'Content-Type: application/json' -d "$login_body" "$BASE/api/v1/auth/login")"
check "POST /api/v1/auth/login (jury)" 200 "${login_resp##*$'\n'}"
CSRF="$(printf '%s' "${login_resp%$'\n'*}" | jq -r '.csrfToken // empty')"
[ -n "$CSRF" ] && ok "login вернул csrfToken" || bad "login вернул csrfToken" "нет поля"

get() { curl -s -b "$JAR" "$BASE$1"; }
gcode() { code -b "$JAR" "$BASE$1"; }
check "GET /api/v1/auth/me" 200 "$(gcode /api/v1/auth/me)"
check "GET /api/v1/analytics/summary" 200 "$(gcode /api/v1/analytics/summary)"
check "GET /api/v1/model/settings" 200 "$(gcode /api/v1/model/settings)"
check "GET /customers?limit=5" 200 "$(gcode '/api/v1/customers?limit=5')"

first_id="$(get '/api/v1/customers?limit=1' | jq -r '.items[0].id // empty')"
if [ -n "$first_id" ]; then
  check "GET /customers/{id}" 200 "$(gcode "/api/v1/customers/$first_id")"
else
  bad "GET /customers/{id}" "очередь пуста, нет id"
fi

# сортировки по каждому полю в обе стороны: порядок проверяем по ответу
for field in id email balance totalReplenished; do
  for order in asc desc; do
    vals="$(get "/api/v1/customers?sort=$field&order=$order&limit=100" | jq -c "[.items[].$field]")"
    if [ -z "$vals" ] || [ "$vals" = "[]" ] || [ "$vals" = "null" ]; then
      bad "сортировка $field $order" "пустой ответ"
      continue
    fi
    if [ "$order" = asc ]; then expr='. == sort'; else expr='. == (sort | reverse)'; fi
    if printf '%s' "$vals" | jq -e "$expr" >/dev/null; then
      ok "сортировка $field $order"
    else
      bad "сортировка $field $order" "порядок нарушен"
    fi
  done
done

# /metrics по токену
check "GET /metrics по токену" 200 "$(code -H "Authorization: Bearer $(envval METRICS_TOKEN)" "$BASE/metrics")"

# PUT /source/snapshot по сервисному токену: один счёт, одна услуга, один платёж
snap='{"accounts":[{"id":900001,"email":"quickstart@example.com","balance":1000,"withdrawBalance":0,"totalReplenished":5000,"bonusBalance":0,"bonusState":"locked"}],
"services":[{"id":900001,"name":"vps","ownerId":900001,"paymentTerm":"month","autoProlong":true,"createdAt":"2026-01-01T00:00:00Z","expiresAt":"2030-01-01T00:00:00Z","status":"active","price":500}],
"transactions":[{"id":900001,"amount":500,"status":"performed","createdAt":"2026-01-01T00:00:00Z","performedAt":"2026-01-01T00:00:00Z","type":"order","serviceId":900001}]}'
check "PUT /api/v1/source/snapshot по токену" 200 "$(code -X PUT -H "Authorization: Bearer $(envval SOURCE_TOKEN)" \
  -H 'Content-Type: application/json' -d "$snap" "$BASE/api/v1/source/snapshot")"

# --- 4. Выход и настройки после перезапуска
mut() { curl -s -b "$JAR" -H 'Content-Type: application/json' -H "X-CSRF-Token: $CSRF" "$@"; }
want=0.37
code_patch="$(mut -o /dev/null -w '%{http_code}' -X PATCH -d "{\"weights\":{\"noauto\":$want}}" "$BASE/api/v1/model/settings")"
check "PATCH /api/v1/model/settings" 200 "$code_patch"
docker compose restart app >/dev/null 2>&1 || bad "docker compose restart app" "ошибка"
for _ in $(seq 1 60); do
  [ "$(code "$BASE/healthz")" = 200 ] && break
  sleep 2
done
# сессии могут не пережить перезапуск — входим заново
CSRF="$(curl -s -c "$JAR" -H 'Content-Type: application/json' -d "$login_body" "$BASE/api/v1/auth/login" | jq -r '.csrfToken // empty')"
got="$(get /api/v1/model/settings | jq -r '.weights.noauto // empty')"
if [ -n "$got" ] && [ "$(jq -n "$got == $want")" = true ]; then
  ok "настройки модели пережили restart app"
else
  bad "настройки модели пережили restart app" "noauto=$got, ждали $want"
fi

check "POST /api/v1/auth/logout" 204 "$(code -b "$JAR" -X POST -H "X-CSRF-Token: $CSRF" "$BASE/api/v1/auth/logout")"
check "после logout 401" 401 "$(gcode /api/v1/auth/me)"

echo
[ "$FAILS" -eq 0 ] && echo "все проверки пройдены" || echo "провалено проверок: $FAILS"
exit $((FAILS > 0))
