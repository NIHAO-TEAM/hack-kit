#!/usr/bin/env bash
# Падение базы под залпом: docs/load/outage.sh [N] (по умолчанию 10000 событий).
# Поток rate=0 через POST /source/emulator/stream; через OUTAGE_AFTER секунд PostgreSQL ставится на паузу
# (docker compose pause) на OUTAGE_SECONDS секунд, затем возвращается. Пока базы нет, раз в секунду
# проверяются /healthz, /readyz, чтение очереди с сессией из кэша и запрос с незнакомой сессией.
# Красный (код 1), если нарушено: процесс перезапускался, /healthz не 200, /readyz не 503, чтение очереди
# отвечает ошибкой, запрос с промахом кэша даёт 500 (нужен 503 с Retry-After) или нет 401/503,
# applied+failed≠events, согласованность≠0 центов, /readyz не вернулся в 200 после возврата базы.
#
# Среда: BASE_URL (http://localhost:8080), LOGIN (admin), ADMIN_PASSWORD, METRICS_TOKEN, SOURCE_TOKEN + SNAPSHOT,
#        OUT (./outage-out), SEED (1), APP_CONTAINER, DB_SERVICE (postgres), OUTAGE_AFTER (3), OUTAGE_SECONDS (10),
#        OUTAGE_MODE (pause | kill — docker compose pause или stop/start).
set -euo pipefail

BASE_URL=${BASE_URL:-http://localhost:8080}
API="$BASE_URL/api/v1"
LOGIN=${LOGIN:-admin}
OUT=${OUT:-outage-out}
SEED=${SEED:-1}
N=${1:-10000}
DB_SERVICE=${DB_SERVICE:-postgres}
OUTAGE_AFTER=${OUTAGE_AFTER:-3}
OUTAGE_SECONDS=${OUTAGE_SECONDS:-10}
OUTAGE_MODE=${OUTAGE_MODE:-pause}
: "${ADMIN_PASSWORD:?не задан ADMIN_PASSWORD}"
: "${METRICS_TOKEN:?не задан METRICS_TOKEN}"
mkdir -p "$OUT"
FAIL=0

curl_api() { curl -fsS --max-time 60 "$@"; }
metrics() { curl -fsS --max-time 20 -H "Authorization: Bearer $METRICS_TOKEN" "$BASE_URL/metrics"; }
metric() { awk -v n="$1" '$1==n {print $2; exit}'; }
fail() { echo "::error::$*" >&2; FAIL=1; }
# code URL [заголовки curl...] — только код ответа; 000 — нет ответа.
code() { local u=$1; shift; curl -s -o /dev/null --max-time 5 -w '%{http_code}' "$@" "$u" || true; }
restart_count() {
  [ -n "${APP_CONTAINER:-}" ] && command -v docker >/dev/null && docker inspect -f '{{.RestartCount}}' "$APP_CONTAINER" 2>/dev/null || echo ""
}

if [ -n "${SOURCE_TOKEN:-}" ] && [ -n "${SNAPSHOT:-}" ]; then
  echo "Загрузка снимка $SNAPSHOT"
  curl -fsS --max-time 300 -X PUT "$API/source/snapshot" -H "Content-Type: application/json" \
    -H "Authorization: Bearer $SOURCE_TOKEN" --data-binary "@$SNAPSHOT" -o /dev/null
fi

TOKEN=$(curl_api -X POST "$API/auth/login" -H 'Content-Type: application/json' \
  -d "{\"login\":\"$LOGIN\",\"password\":\"$ADMIN_PASSWORD\"}" | jq -r .token)
echo "::add-mask::$TOKEN"
AUTH="Authorization: Bearer $TOKEN"
curl_api "$API/customers?limit=1" -H "$AUTH" -o /dev/null # сессия попадает в кэш процесса

M0=$(metrics)
START0=$(metric process_start_time_seconds <<<"$M0")
RC0=$(restart_count)

curl_api -X POST "$API/source/emulator/stream" -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"events\":$N,\"ratePerSecond\":0,\"seed\":$SEED}" -o /dev/null
sleep "$OUTAGE_AFTER"
# Новый вход перед отказом: новый токен — гарантированный промах кэша и put, окно TTL считаем от паузы.
TOKEN=$(curl_api -X POST "$API/auth/login" -H 'Content-Type: application/json' \
  -d "{\"login\":\"$LOGIN\",\"password\":\"$ADMIN_PASSWORD\"}" | jq -r .token)
echo "::add-mask::$TOKEN"
AUTH="Authorization: Bearer $TOKEN"
curl_api "$API/customers?limit=1" -H "$AUTH" -o /dev/null

# Пробы пока базы нет: строка на пробу — время, healthz, readyz, чтение с кэшем, чтение без кэша.
: >"$OUT/probes.tsv"
echo "=== База недоступна $OUTAGE_SECONDS с ($OUTAGE_MODE) ==="
if [ "$OUTAGE_MODE" = kill ]; then docker compose stop -t 0 "$DB_SERVICE"; else docker compose pause "$DB_SERVICE"; fi
T0=$SECONDS
T_END=$((SECONDS + OUTAGE_SECONDS))
NOCACHE=""
# Кэш сессии живёт SESSION_CACHE_TTL (5 с по умолчанию) и не продлевается: 200 требуем только в его пределах.
CACHE_TTL=${SESSION_CACHE_TTL_SECONDS:-5}
while [ $SECONDS -lt $T_END ]; do
  H=$(code "$BASE_URL/healthz"); R=$(code "$BASE_URL/readyz")
  C=$(code "$API/customers?limit=1" -H "$AUTH")
  # Незнакомый токен: при недоступной БД ждём 503 + Retry-After (401 бывает, если проверка успела до отказа).
  NC=$(curl -s -o /dev/null --max-time 10 -D "$OUT/nocache.hdr" -w '%{http_code}' -H "Authorization: Bearer outage-$RANDOM$RANDOM" "$API/customers?limit=1" || true)
  RA=$(awk 'tolower($1)=="retry-after:" {print $2}' "$OUT/nocache.hdr" | tr -d '\r')
  printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$SECONDS" "$H" "$R" "$C" "$NC" "$RA" >>"$OUT/probes.tsv"
  NOCACHE="$NOCACHE $NC"
  sleep 1
done
if [ "$OUTAGE_MODE" = kill ]; then docker compose start "$DB_SERVICE"; else docker compose unpause "$DB_SERVICE"; fi
echo "=== База вернулась ==="

DEADLINE=$((SECONDS + 1200))
while :; do
  STATE=$(curl_api "$API/source/emulator/stream" -H "$AUTH" 2>/dev/null || echo '{"state":"running"}')
  S=$(jq -r .state <<<"$STATE")
  [ "$S" = running ] || break
  [ $SECONDS -lt $DEADLINE ] || { fail "прогон не завершился за 20 минут"; break; }
  sleep 2
done
jq . <<<"$STATE" >"$OUT/stream.json"

READY_AFTER=$(code "$BASE_URL/readyz")
M1=$(metrics)
START1=$(metric process_start_time_seconds <<<"$M1")
RC1=$(restart_count)
WAITS=$(awk '/^churn_ingest_db_unavailable_total/ {print $2; exit}' <<<"$M1")

PROBES=$(wc -l <"$OUT/probes.tsv")
[ "$PROBES" -gt 0 ] || fail "ни одной пробы за время отказа"
BAD_H=$(awk -F'\t' '$2!=200' "$OUT/probes.tsv" | wc -l)
BAD_R=$(awk -F'\t' '$3!=503' "$OUT/probes.tsv" | wc -l)
BAD_C=$(awk -F'\t' -v t0="$T0" -v ttl="$CACHE_TTL" '$1-t0 < ttl-2 && $4!=200' "$OUT/probes.tsv" | wc -l)
BAD_C2=$(awk -F'\t' -v t0="$T0" -v ttl="$CACHE_TTL" '$1-t0 >= ttl-2 && $4!=200 && $4!=503' "$OUT/probes.tsv" | wc -l)
N500=$(awk -F'\t' '$5>=500 && $5!=503 || $4>=500' "$OUT/probes.tsv" | wc -l)
BAD_NC=$(awk -F'\t' '$5!=401 && !($5==503 && $6!="")' "$OUT/probes.tsv" | wc -l)
N503=$(awk -F'\t' '$5==503' "$OUT/probes.tsv" | wc -l)
[ "$BAD_H" = 0 ] || fail "/healthz не 200 в $BAD_H пробах"
[ "$BAD_R" = 0 ] || fail "/readyz не 503 в $BAD_R пробах"
[ "$BAD_C" = 0 ] || fail "чтение очереди с кэшем сессии не 200 в пределах TTL кэша в $BAD_C пробах"
[ "$BAD_C2" = 0 ] || fail "чтение после TTL кэша не 200 и не 503 в $BAD_C2 пробах (зависание или 500)"
[ "$N500" = 0 ] || fail "ответов 500 при недоступной базе: $N500"
[ "$BAD_NC" = 0 ] || fail "запрос с промахом кэша не 401 и не 503 с Retry-After в $BAD_NC пробах"
[ "$READY_AFTER" = 200 ] || fail "/readyz после возврата базы: $READY_AFTER"
[ "$START0" = "$START1" ] || fail "процесс перезапускался (process_start_time_seconds изменился)"
[ -z "$RC0$RC1" ] || [ "$RC0" = "$RC1" ] || fail "restartCount $RC0 → $RC1"
[ "$S" = done ] || fail "состояние прогона «$S», ошибка: $(jq -r '.error // ""' <<<"$STATE")"
APPLIED=$(jq -r '.report.applied // 0' <<<"$STATE"); FAILED=$(jq -r '.report.failed // 0' <<<"$STATE")
[ $((APPLIED + FAILED)) -eq "$N" ] || fail "применено $APPLIED + отклонено $FAILED ≠ $N"
[ "$FAILED" = 0 ] || fail "отклонено $FAILED событий: при отказе базы события не должны теряться"
[ "$(jq -r '.report.consistency.ok // false' <<<"$STATE")" = true ] || fail "согласованность не сошлась"
[ "$(jq -r '.report.consistency.diffCents // -1' <<<"$STATE")" = 0 ] || fail "разница с пересчётом с нуля не 0 центов"

jq -n --argjson n "$N" --argjson secs "$OUTAGE_SECONDS" --arg mode "$OUTAGE_MODE" --argjson probes "$PROBES" \
  --argjson n503 "$N503" --argjson n500 "$N500" --arg waits "${WAITS:-0}" --argjson r "$(jq '.report // {}' <<<"$STATE")" \
  '{events:$n, outageSeconds:$secs, mode:$mode, probes:$probes, noCache503:$n503, http500:$n500, dbWaits:$waits,
    applied:$r.applied, failed:$r.failed, eventsPerSecond:$r.eventsPerSecond, consistency:$r.consistency}' >"$OUT/outage.json"
cat "$OUT/outage.json"
[ -z "${GITHUB_STEP_SUMMARY:-}" ] || { echo "## Падение базы под залпом"; echo '```json'; cat "$OUT/outage.json"; echo '```'; } >>"$GITHUB_STEP_SUMMARY"

[ $FAIL = 0 ] || { echo "Отказ базы не прошёл проверки" >&2; exit 1; }
echo "Все проверки отказа базы пройдены"
