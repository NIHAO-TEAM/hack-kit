#!/usr/bin/env bash
# Под убит под залпом (изменение pod-kill-under-load): docs/load/chaos.sh
# Две реплики API в k3d. Этап A: залп событий со скоростью RATE/с, читатели очереди и SSE-клиенты,
# во время залпа удаляется под, который НЕ ведёт поток. Этап B: читатели без залпа, `rollout restart` под нагрузкой.
# Красный (код 1), если нарушено: 5xx у читателей, залп не дошёл до done, согласованность ≠ 0 центов,
# сводки реплик расходятся после пересчёта, SSE-клиенты не переподключились.
# Скорость ограничена (по умолчанию 400/с): на полной скорости залп 10 000 идёт меньше секунды, убийство не попало бы в него.
#
# Среда: BASE_URL, HOST (имя в Ingress), LOGIN (admin), ADMIN_PASSWORD, SOURCE_TOKEN, SNAPSHOT (data.json),
#        NS (churn), RELEASE (churn), EVENTS (10000), RATE (400), OUT (chaos-out), PG_USER и PG_DB (app).
set -euo pipefail

BASE_URL=${BASE_URL:?не задан BASE_URL}
API="$BASE_URL/api/v1"
LOGIN=${LOGIN:-admin}
NS=${NS:-churn}
RELEASE=${RELEASE:-churn}
EVENTS=${EVENTS:-10000}
RATE=${RATE:-400}
OUT=${OUT:-chaos-out}
HERE=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
: "${ADMIN_PASSWORD:?не задан ADMIN_PASSWORD}"
mkdir -p "$OUT"
FAIL=0
fail() { echo "::error::$*" >&2; FAIL=1; }
curl_api() { curl -fsS --max-time 60 "$@"; }
# Реплики, которые сейчас Ready (под без deletionTimestamp).
ready_pods() {
  kubectl -n "$NS" get pods -l app.kubernetes.io/component=app -o json \
    | jq -r '.items[] | select(.metadata.deletionTimestamp == null) | select(any(.status.conditions[]?; .type == "Ready" and .status == "True")) | .metadata.name'
}

if [ -n "${SOURCE_TOKEN:-}" ] && [ -n "${SNAPSHOT:-}" ]; then
  echo "Загрузка снимка $SNAPSHOT"
  # Код, заголовки ответа и время пишем в $OUT (токен в них не попадает): по ним видно, кто вернул 401, под или Traefik.
  UP_CODE=$(curl -sS --max-time 600 -X PUT "$API/source/snapshot" -H "Content-Type: application/json" \
    -H "Authorization: Bearer $SOURCE_TOKEN" --data-binary "@$SNAPSHOT" -D "$OUT/snapshot-headers.txt" -o "$OUT/snapshot-body.txt" \
    -w '%{http_code} total=%{time_total}s sent=%{size_upload}' || true)
  echo "загрузка снимка: $UP_CODE"
  case "$UP_CODE" in
    2*) ;;
    *) cat "$OUT/snapshot-headers.txt" "$OUT/snapshot-body.txt" >&2 || true
       for p in $(kubectl -n "$NS" get pods -l app.kubernetes.io/component=app -o name); do
         echo "--- лог $p" >&2; kubectl -n "$NS" logs "$p" --tail=40 >&2 || true
       done
       echo "--- лог Traefik" >&2; kubectl -n kube-system logs deploy/traefik --tail=40 >&2 || true
       fail "загрузка снимка вернула $UP_CODE"; exit 1 ;;
  esac
fi
TOKEN=$(curl_api -X POST "$API/auth/login" -H 'Content-Type: application/json' \
  -d "{\"login\":\"$LOGIN\",\"password\":\"$ADMIN_PASSWORD\"}" | jq -r .token) \
  || { fail "вход под $LOGIN не прошёл (401: проверь, что в Secret заданы ADMIN_LOGIN и ADMIN_PASSWORD)"; exit 1; }
echo "::add-mask::$TOKEN"
AUTH="Authorization: Bearer $TOKEN"

[ "$(ready_pods | wc -l)" -ge 2 ] || { fail "нет двух Ready-реплик до начала"; exit 1; }

# wait_calc — после загрузки снимка обе реплики пересчитывают его (≈20–30 с на 100 000 клиентов); пока у реплики нет
# результата, она честно отвечает 503 «расчёт ещё не готов» (под при этом Ready: на пустой базе иначе нельзя
# загрузить первый снимок, cmd/api/main.go ready). Читатели стартуют, когда 20 запросов подряд к Service вернули 200:
# при двух репликах это значит, что готовы обе. Порог ошибок чтения не меняется: он считается с этого момента.
wait_calc() {
  local t0=$SECONDS ok=0
  while [ $ok -lt 20 ]; do
    if [ "$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$API/customers?limit=1" -H "$AUTH")" = 200 ]; then ok=$((ok + 1)); else ok=0; sleep 1; fi
    [ $((SECONDS - t0)) -lt 300 ] || { fail "расчёт после загрузки снимка не готов за 5 минут"; exit 1; }
  done
  echo "расчёт после загрузки снимка готов на обеих репликах через $((SECONDS - t0)) с"
}

start_readers() { # $1 — метка этапа; k6 идёт, пока ему не пошлют SIGINT
  K6PIDS=()
  k6 run --quiet --summary-export "$OUT/k6-read-$1.json" -e PASSWORD="$ADMIN_PASSWORD" -e LOGIN="$LOGIN" -e BASE_URL="$BASE_URL" \
    "$HERE/k6/burst-read.js" >"$OUT/k6-read-$1.log" 2>&1 & K6PIDS+=($!)
  k6 run --quiet --summary-export "$OUT/k6-sse-$1.json" -e PASSWORD="$ADMIN_PASSWORD" -e LOGIN="$LOGIN" -e BASE_URL="$BASE_URL" -e CLIENTS="${SSE_CLIENTS:-200}" \
    "$HERE/k6/sse.js" >"$OUT/k6-sse-$1.log" 2>&1 & K6PIDS+=($!)
  sleep 15 # SSE-клиенты успевают подключиться
}
stop_readers() {
  for p in "${K6PIDS[@]}"; do kill -INT "$p" 2>/dev/null || true; done
  for p in "${K6PIDS[@]}"; do wait "$p" 2>/dev/null || true; done
}
# check_readers ЭТАП — 5xx и ошибки чтения, повторы после 503 на /readyz, обрывы SSE (считаем отдельно, не ошибка).
check_readers() {
  local f="$OUT/k6-read-$1.json" s="$OUT/k6-sse-$1.json"
  READ_FAIL=$(jq -r '.metrics.http_req_failed.value // 0' "$f")
  READ_N=$(jq -r '.metrics.http_reqs.count // 0' "$f")
  SSE_DROPS=$(jq -r '.metrics.sse_drops.count // 0' "$s")
  SSE_ALIVE=$(jq -r '.metrics.sse_window_alive.value // 0' "$s")
  echo "этап $1: запросов чтения $READ_N, доля ошибок $READ_FAIL, обрывов SSE (переподключены) $SSE_DROPS, живых окон SSE $SSE_ALIVE"
  if [ "$READ_FAIL" != 0 ]; then
    echo "этап $1: ошибки чтения по коду ответа (0 — оборванный запрос):"
    jq -r '.metrics | to_entries[] | select(.key | startswith("read_err_")) | select(.value.count > 0) | "  \(.key | sub("read_err_"; "")): \(.value.count)"' "$f"
    echo "этап $1: ошибки k6 (первые 10 и по одной в секунду):"
    grep "ошибка чтения" "$OUT/k6-read-$1.log" | head -60 || true
  fi
  [ "$READ_N" -gt 0 ] || fail "этап $1: читатели не сделали ни одного запроса"
  [ "$READ_FAIL" = 0 ] || fail "этап $1: ошибок чтения очереди (5xx и обрывы соединения): доля $READ_FAIL"
  # SSE обязаны вернуться: после обрыва есть живые окна, то есть клиенты переподключились к оставшейся реплике.
  [ "$(awk -v a="$SSE_ALIVE" 'BEGIN {print (a > 0) ? 1 : 0}')" = 1 ] || fail "этап $1: SSE-клиенты не переподключились"
}

# dump_pods МЕТКА — состояние реплик: Ready, перезапуски, причина последнего падения контейнера и конец его лога.
dump_pods() {
  echo "--- реплики ($1) $(date -u +%T)"
  kubectl -n "$NS" get pods -l app.kubernetes.io/component=app -o wide || true
  kubectl -n "$NS" get pods -l app.kubernetes.io/component=app -o json | jq -r '.items[] | "\(.metadata.name) ready=\(any(.status.conditions[]?; .type == "Ready" and .status == "True")) перезапусков=\(.status.containerStatuses[0].restartCount) прошлое падение: \(.status.containerStatuses[0].lastState.terminated | if . then "\(.reason) код \(.exitCode)" else "нет" end)"' || true
}
dump_prev_logs() {
  for p in $(kubectl -n "$NS" get pods -l app.kubernetes.io/component=app -o name); do
    echo "--- лог $p: пересчёты и «портфель не собран»"; kubectl -n "$NS" logs "$p" 2>&1 | grep -E "портфель не собран|пересчёт|поток" | head -40 || true
  done
  for p in $(kubectl -n "$NS" get pods -l app.kubernetes.io/component=app -o name); do
    echo "--- прошлый контейнер $p"; kubectl -n "$NS" logs "$p" --previous --tail=25 2>&1 || true
  done
}

# ---- Этап A: убийство пода под залпом ----
echo "=== Этап A: залп $EVENTS событий, $RATE/с, удаление пода без потока ==="
wait_calc
start_readers A
curl_api -X POST "$API/source/emulator/stream" -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"events\":$EVENTS,\"ratePerSecond\":$RATE,\"seed\":1}" -o /dev/null
sleep 3

# Ведущая реплика держит advisory-блокировку на соединении с базой: адрес клиента этого соединения — IP её пода.
LEAD_IP=$(kubectl -n "$NS" exec "$RELEASE-postgres-0" -- psql -U "${PG_USER:-app}" -d "${PG_DB:-app}" -Atc \
  "SELECT a.client_addr FROM pg_locks l JOIN pg_stat_activity a ON a.pid = l.pid WHERE l.locktype = 'advisory' AND l.classid::bigint * 4294967296 + l.objid::bigint = 7405120001 LIMIT 1")
[ -n "$LEAD_IP" ] || { fail "этап A: не нашёл реплику, которая ведёт поток"; stop_readers; exit 1; }
LEAD=$(kubectl -n "$NS" get pods -l app.kubernetes.io/component=app -o json | jq -r --arg ip "$LEAD_IP" '.items[] | select(.status.podIP == $ip) | .metadata.name')
VICTIM=$(ready_pods | grep -vx "$LEAD" | head -1)
[ -n "$VICTIM" ] || { fail "этап A: нет второй реплики для удаления"; stop_readers; exit 1; }
echo "ведёт поток: $LEAD; удаляю: $VICTIM"
kubectl -n "$NS" delete pod "$VICTIM" --wait=false
for t in 5 10 20; do sleep "$t"; dump_pods "через $t с после удаления"; done

DEADLINE=$((SECONDS + 600))
while :; do
  STATE=$(curl_api "$API/source/emulator/stream" -H "$AUTH")
  S=$(jq -r .state <<<"$STATE")
  [ "$S" = running ] || break
  [ $SECONDS -lt $DEADLINE ] || { fail "этап A: прогон не завершился за 10 минут"; break; }
  sleep 2
done
kubectl -n "$NS" rollout status "deploy/$RELEASE-app" --timeout=300s
dump_pods "конец этапа A"; dump_prev_logs
stop_readers
jq . <<<"$STATE" >"$OUT/stream-A.json"
[ "$S" = done ] || fail "этап A: состояние прогона «$S», ошибка: $(jq -r '.error // ""' <<<"$STATE")"
APPLIED=$(jq -r '.report.applied // 0' <<<"$STATE"); FAILED=$(jq -r '.report.failed // 0' <<<"$STATE")
[ $((APPLIED + FAILED)) -eq "$EVENTS" ] || fail "этап A: применено $APPLIED + отклонено $FAILED ≠ $EVENTS"
[ "$(jq -r '.report.consistency.diffCents // -1' <<<"$STATE")" = 0 ] || fail "этап A: разница с пересчётом с нуля не 0 центов"
check_readers A

# ---- Этап B: выкат под нагрузкой чтения ----
echo "=== Этап B: rollout restart под чтением ==="
start_readers B
kubectl -n "$NS" rollout restart "deploy/$RELEASE-app"
kubectl -n "$NS" rollout status "deploy/$RELEASE-app" --timeout=600s
sleep 10
stop_readers
check_readers B

# ---- Сводки обеих реплик равны ----
echo "=== Сводки реплик ==="
sleep 15 # пересчёт на каждой реплике догоняет общую БД
n=0
for pod in $(ready_pods); do
  n=$((n + 1))
  kubectl -n "$NS" port-forward "pod/$pod" "$((18080 + n)):8080" >/dev/null 2>&1 & PF=$!
  sleep 3
  S_TOKEN=$(curl_api -X POST "http://127.0.0.1:$((18080 + n))/api/v1/auth/login" -H 'Content-Type: application/json' \
    -d "{\"login\":\"$LOGIN\",\"password\":\"$ADMIN_PASSWORD\"}" | jq -r .token)
  curl_api "http://127.0.0.1:$((18080 + n))/api/v1/analytics/summary" -H "Authorization: Bearer $S_TOKEN" | jq -S . >"$OUT/summary-$n.json" || fail "сводка $pod не прочитана"
  kill $PF 2>/dev/null || true
done
[ "$n" -ge 2 ] || fail "после выката меньше двух реплик"
[ "$n" -lt 2 ] || cmp -s "$OUT/summary-1.json" "$OUT/summary-2.json" || fail "сводки реплик расходятся (diff в $OUT)"

{
  echo "## Под убит под залпом"
  echo "- залп: $EVENTS событий, $RATE/с, применено $APPLIED, отклонено $FAILED"
  echo "- согласованность: разница $(jq -r '.report.consistency.diffCents' "$OUT/stream-A.json") центов"
  echo "- читатели этап A: $(jq -r '.metrics.http_reqs.count' "$OUT/k6-read-A.json") запросов, доля ошибок $(jq -r '.metrics.http_req_failed.value' "$OUT/k6-read-A.json")"
  echo "- читатели этап B (выкат): $(jq -r '.metrics.http_reqs.count' "$OUT/k6-read-B.json") запросов, доля ошибок $(jq -r '.metrics.http_req_failed.value' "$OUT/k6-read-B.json")"
} | tee "$OUT/summary.md"
[ -z "${GITHUB_STEP_SUMMARY:-}" ] || cat "$OUT/summary.md" >>"$GITHUB_STEP_SUMMARY"
[ $FAIL = 0 ] || { echo "Прогон не прошёл проверки" >&2; exit 1; }
echo "Все проверки пройдены"
