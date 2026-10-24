#!/usr/bin/env bash
# Залп событий эмулятора по стенду: docs/load/burst.sh [N ...] (по умолчанию 3000 10000 50000).
# На каждый залп: поток rate=0 через POST /source/emulator/stream, опрос до done, проверки, строка итога.
# Если k6 есть в PATH (и не задан BURST_NO_K6=1), параллельно идут чтение очереди (20 VU) и 500 SSE-клиентов.
# Число событий, сброшенных шиной медленным подписчикам, пишется в отчёт (busDropped), но проверкой не является.
# Красный (код 1), если нарушено: 5xx, applied+failed≠events, согласованность≠0 центов, рестарт процесса, обрыв SSE.
#
# Среда: BASE_URL (http://localhost:8080), LOGIN (admin), ADMIN_PASSWORD, METRICS_TOKEN,
#        SOURCE_TOKEN + SNAPSHOT (путь к data.json; если заданы — снимок грузится перед залпами),
#        OUT (каталог результатов, ./burst-out), SEED (1), APP_CONTAINER (имя контейнера для restartCount).
set -euo pipefail

BASE_URL=${BASE_URL:-http://localhost:8080}
API="$BASE_URL/api/v1"
LOGIN=${LOGIN:-admin}
OUT=${OUT:-burst-out}
SEED=${SEED:-1}
HERE=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
: "${ADMIN_PASSWORD:?не задан ADMIN_PASSWORD}"
: "${METRICS_TOKEN:?не задан METRICS_TOKEN}"
SIZES=("$@")
[ ${#SIZES[@]} -gt 0 ] || SIZES=(3000 10000 50000)
mkdir -p "$OUT"
FAIL=0

curl_api() { curl -fsS --max-time 60 "$@"; }
metrics() { curl -fsS --max-time 20 -H "Authorization: Bearer $METRICS_TOKEN" "$BASE_URL/metrics"; }
# metric NAME — значение метрики без меток из текста /metrics на stdin.
metric() { awk -v n="$1" '$1==n {print $2; exit}'; }
# count5xx — сколько ответов 5xx посчитал сервер (по гистограмме длительности).
count5xx() { awk '/^churn_http_request_duration_seconds_count\{/ && /code="5[0-9][0-9]"/ {s+=$2} END {print s+0}'; }
# countDropped — сколько событий шина сбросила медленным подписчикам (сумма churn_events_dropped_total по меткам).
countDropped() { awk '/^churn_events_dropped_total(\{| )/ {s+=$NF} END {print s+0}'; }
fail() { echo "::error::$*" >&2; FAIL=1; }

if [ -n "${SOURCE_TOKEN:-}" ] && [ -n "${SNAPSHOT:-}" ]; then
  echo "Загрузка снимка $SNAPSHOT"
  curl -fsS --max-time 300 -X PUT "$API/source/snapshot" -H "Content-Type: application/json" \
    -H "Authorization: Bearer $SOURCE_TOKEN" --data-binary "@$SNAPSHOT" -o /dev/null
fi

TOKEN=$(curl_api -X POST "$API/auth/login" -H 'Content-Type: application/json' \
  -d "{\"login\":\"$LOGIN\",\"password\":\"$ADMIN_PASSWORD\"}" | jq -r .token)
echo "::add-mask::$TOKEN"
AUTH="Authorization: Bearer $TOKEN"

HAVE_K6=0
if [ -z "${BURST_NO_K6:-}" ] && command -v k6 >/dev/null; then HAVE_K6=1; fi

restart_count() {
  [ -n "${APP_CONTAINER:-}" ] && command -v docker >/dev/null && docker inspect -f '{{.RestartCount}}' "$APP_CONTAINER" 2>/dev/null || echo ""
}

for N in "${SIZES[@]}"; do
  echo "=== Залп $N событий ==="
  M0=$(metrics)
  START0=$(metric process_start_time_seconds <<<"$M0")
  FIVE0=$(count5xx <<<"$M0")
  DROP0=$(countDropped <<<"$M0")
  RC0=$(restart_count)

  # Пик памяти и горутин — раз в секунду, пока идёт залп.
  : >"$OUT/samples-$N.tsv"
  ( while :; do
      m=$(metrics 2>/dev/null) || { sleep 1; continue; }
      printf '%s\t%s\n' "$(metric process_resident_memory_bytes <<<"$m")" "$(metric go_goroutines <<<"$m")" >>"$OUT/samples-$N.tsv"
      sleep 1
    done ) &
  SAMPLER=$!

  K6PIDS=()
  if [ $HAVE_K6 = 1 ]; then
    k6 run --quiet --summary-export "$OUT/k6-read-$N.json" -e PASSWORD="$ADMIN_PASSWORD" -e LOGIN="$LOGIN" -e BASE_URL="$BASE_URL" \
      "$HERE/k6/burst-read.js" >"$OUT/k6-read-$N.log" 2>&1 & K6PIDS+=($!)
    k6 run --quiet --summary-export "$OUT/k6-sse-$N.json" -e PASSWORD="$ADMIN_PASSWORD" -e LOGIN="$LOGIN" -e BASE_URL="$BASE_URL" \
      "$HERE/k6/sse.js" >"$OUT/k6-sse-$N.log" 2>&1 & K6PIDS+=($!)
    sleep 15 # SSE-клиенты успевают подключиться до первого события
  fi

  curl_api -X POST "$API/source/emulator/stream" -H "$AUTH" -H 'Content-Type: application/json' \
    -d "{\"events\":$N,\"ratePerSecond\":0,\"seed\":$SEED}" -o /dev/null
  DEADLINE=$((SECONDS + 1200))
  while :; do
    STATE=$(curl_api "$API/source/emulator/stream" -H "$AUTH")
    S=$(jq -r .state <<<"$STATE")
    [ "$S" = running ] || break
    [ $SECONDS -lt $DEADLINE ] || { fail "залп $N: прогон не завершился за 20 минут"; break; }
    sleep 2
  done

  # Корректное завершение k6: SIGINT — окна SSE и чтения дорабатывают, сводка пишется.
  for p in "${K6PIDS[@]:-}"; do [ -n "$p" ] && kill -INT "$p" 2>/dev/null || true; done
  for p in "${K6PIDS[@]:-}"; do [ -n "$p" ] && wait "$p" 2>/dev/null || true; done
  kill "$SAMPLER" 2>/dev/null || true; wait "$SAMPLER" 2>/dev/null || true

  M1=$(metrics)
  START1=$(metric process_start_time_seconds <<<"$M1")
  FIVE1=$(count5xx <<<"$M1")
  DROP1=$(countDropped <<<"$M1")
  RC1=$(restart_count)
  jq . <<<"$STATE" >"$OUT/stream-$N.json"

  APPLIED=$(jq -r '.report.applied // 0' <<<"$STATE"); FAILED=$(jq -r '.report.failed // 0' <<<"$STATE")
  if [ "$S" = done ]; then
    [ $((APPLIED + FAILED)) -eq "$N" ] || fail "залп $N: применено $APPLIED + отклонено $FAILED ≠ $N"
    [ "$(jq -r '.report.consistency.ok // false' <<<"$STATE")" = true ] || fail "залп $N: согласованность не сошлась"
    [ "$(jq -r '.report.consistency.diffCents // -1' <<<"$STATE")" = 0 ] || fail "залп $N: разница с пересчётом с нуля не 0 центов"
  else
    # итога нет: сводку по нулям не печатаем, показываем причину и то, сколько событий успели обработать
    fail "залп $N: состояние прогона «$S», обработано $(jq -r '.applied // 0' <<<"$STATE") из $N, причина сбоя: $(jq -r '.error // "не указана"' <<<"$STATE")"
  fi
  [ "$FIVE1" = "$FIVE0" ] || fail "залп $N: ответов 5xx за залп: $((FIVE1 - FIVE0))"
  [ "$START0" = "$START1" ] || fail "залп $N: процесс перезапускался (process_start_time_seconds изменился)"
  [ -z "$RC0$RC1" ] || [ "$RC0" = "$RC1" ] || fail "залп $N: restartCount $RC0 → $RC1"

  DROPS="н/д"; READ_P95="н/д"; READ_FAIL="н/д"
  if [ -f "$OUT/k6-sse-$N.json" ]; then
    DROPS=$(jq -r '.metrics.sse_drops.count // 0' "$OUT/k6-sse-$N.json")
    [ "$DROPS" = 0 ] || fail "залп $N: обрывов SSE: $DROPS"
  fi
  if [ -f "$OUT/k6-read-$N.json" ]; then
    READ_P95=$(jq -r '.metrics.queue_read_ms["p(95)"] // "н/д"' "$OUT/k6-read-$N.json")
    READ_FAIL=$(jq -r '.metrics.http_req_failed.value // 0' "$OUT/k6-read-$N.json")
    [ "$READ_FAIL" = 0 ] || fail "залп $N: ошибок чтения очереди: доля $READ_FAIL"
  fi
  PEAK_MB=$(awk -F'\t' '$1+0>m {m=$1+0} END {printf "%.0f", m/1048576}' "$OUT/samples-$N.tsv")
  PEAK_G=$(awk -F'\t' '$2+0>m {m=$2+0} END {print m+0}' "$OUT/samples-$N.tsv")

  jq -n --argjson n "$N" --argjson r "$(jq '.report // {}' <<<"$STATE")" --arg drops "$DROPS" --arg readp95 "$READ_P95" \
    --arg peakmb "$PEAK_MB" --arg peakg "$PEAK_G" --argjson five "$((FIVE1 - FIVE0))" --argjson busdrop "$((DROP1 - DROP0))" \
    '{events:$n, applied:$r.applied, failed:$r.failed, eventsPerSecond:$r.eventsPerSecond, applyP50Ms:$r.applyP50Ms,
      applyP95Ms:$r.applyP95Ms, applyP99Ms:($r.applyP99Ms // null), queueLagP95Ms:$r.queueLagP95Ms, recalcs:$r.recalcs,
      consistency:$r.consistency, http5xx:$five, busDropped:$busdrop, sseDrops:$drops, queueReadP95Ms:$readp95, peakRssMB:$peakmb, peakGoroutines:$peakg}' \
    >"$OUT/burst-$N.json"
  cat "$OUT/burst-$N.json"
done

# Таблица для summary и отчёта.
{
  echo "| Залп | события/с | p50 мс | p95 мс | p99 мс | «событие → очередь» p95 мс | чтение очереди p95 мс | пересчётов | 5xx | потеряно шиной | обрывов SSE | пик памяти МБ | пик горутин |"
  echo "|---|---|---|---|---|---|---|---|---|---|---|---|---|"
  for N in "${SIZES[@]}"; do
    jq -r '"| \(.events) | \(.eventsPerSecond) | \(.applyP50Ms) | \(.applyP95Ms) | \(.applyP99Ms // "н/д") | \(.queueLagP95Ms // "н/д") | \(.queueReadP95Ms) | \(.recalcs) | \(.http5xx) | \(.busDropped) | \(.sseDrops) | \(.peakRssMB) | \(.peakGoroutines) |"' "$OUT/burst-$N.json"
  done
} | tee "$OUT/summary.md"
[ -z "${GITHUB_STEP_SUMMARY:-}" ] || { echo "## Залпы событий"; cat "$OUT/summary.md"; } >>"$GITHUB_STEP_SUMMARY"

[ $FAIL = 0 ] || { echo "Залп не прошёл проверки" >&2; exit 1; }
echo "Все проверки залпа пройдены"
