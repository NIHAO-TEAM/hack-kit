#!/usr/bin/env bash
# Миллион клиентов на одной машине: docs/load/scale.sh [КЛИЕНТОВ] (по умолчанию 1000000).
# Шаги: генерация данных, загрузка снимка потоком, полный пересчёт (PATCH /model/settings), чтение очереди
# (k6/read.js, 10 VU), залп 10 000 событий (docs/load/burst.sh). Пик памяти приложения и PostgreSQL — раз в секунду.
# Прогон — замер, а не гейт: красные числа попадают в отчёт, выход ненулевой только если шаг не удалось выполнить.
#
# Среда: BASE_URL (http://localhost:8080), LOGIN (admin), ADMIN_PASSWORD, METRICS_TOKEN, SOURCE_TOKEN,
#        OUT (./scale-out), DATA_DIR (каталог данных, $OUT/data), APP_CONTAINER, DB_CONTAINER (имена контейнеров для docker stats).
set -euo pipefail

BASE_URL=${BASE_URL:-http://localhost:8080}
API="$BASE_URL/api/v1"
LOGIN=${LOGIN:-admin}
OUT=${OUT:-scale-out}
ACCOUNTS=${1:-1000000}
DATA_DIR=${DATA_DIR:-$OUT/data}
HERE=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT=$(cd "$HERE/../.." && pwd)
: "${ADMIN_PASSWORD:?не задан ADMIN_PASSWORD}"
: "${METRICS_TOKEN:?не задан METRICS_TOKEN}"
: "${SOURCE_TOKEN:?не задан SOURCE_TOKEN}"
mkdir -p "$OUT" "$DATA_DIR"

metrics() { curl -fsS --max-time 30 -H "Authorization: Bearer $METRICS_TOKEN" "$BASE_URL/metrics"; }
metric() { awk -v n="$1" '$1==n {print $2; exit}'; }
now() { date +%s.%N; }
elapsed() { awk -v a="$1" -v b="$2" 'BEGIN {printf "%.1f", b - a}'; }
note() { echo "$*" | tee -a "$OUT/steps.txt"; }

# Пик памяти: RSS приложения из /metrics и docker stats обоих контейнеров, раз в секунду.
sampler() {
  while :; do
    rss=$(metrics 2>/dev/null | metric process_resident_memory_bytes || true)
    mem=""
    if command -v docker >/dev/null && [ -n "${APP_CONTAINER:-}" ]; then
      mem=$(docker stats --no-stream --format '{{.Name}} {{.MemUsage}}' ${APP_CONTAINER} ${DB_CONTAINER:-} 2>/dev/null | tr '\n' ';' || true)
    fi
    printf '%s\t%s\t%s\n' "$(date +%s)" "${rss:-}" "$mem" >>"$OUT/samples.tsv"
    sleep 1
  done
}
: >"$OUT/samples.tsv"; : >"$OUT/steps.txt"
sampler & SAMPLER=$!
trap 'kill $SAMPLER 2>/dev/null || true' EXIT

# 1. Генерация. cases/analizator/generate.py держит все записи в памяти (на 1M: 14 млн словарей, не влезает в 16 ГБ раннера),
# поэтому снимок строит backend/tools/snapgen: у аккаунта 1–3 услуги, ≈ 2 услуги и ≈ 4 платежа на клиента.
note "Генерация snapgen: клиентов $ACCOUNTS"
(cd "$ROOT/backend" && go build -o "$OUT/snapgen" ./tools/snapgen/cmd)
T0=$(now)
/usr/bin/time -v "$OUT/snapgen" -accounts "$ACCOUNTS" >"$DATA_DIR/data.json" 2>"$OUT/generate-time.txt"
T1=$(now)
SIZE=$(stat -c %s "$DATA_DIR/data.json")
GEN_RSS_KB=$(awk -F: '/Maximum resident set size/ {gsub(/ /,"",$2); print $2}' "$OUT/generate-time.txt")
# Строки считаем по ключам: jq держал бы весь файл в памяти.
ROWS="accounts=$(grep -o '"email"' "$DATA_DIR/data.json" | wc -l) services=$(grep -o '"ownerId"' "$DATA_DIR/data.json" | wc -l) transactions=$(grep -o '"serviceId"' "$DATA_DIR/data.json" | wc -l)"
note "generate_seconds=$(elapsed "$T0" "$T1") file_bytes=$SIZE generate_peak_rss_kb=$GEN_RSS_KB rows=$ROWS"

# 2. Загрузка снимка потоком (curl -T не читает файл в память целиком).
note "Загрузка снимка"
T0=$(now)
CODE=$(curl -sS --max-time 3000 -o "$OUT/upload.json" -w '%{http_code}' -X PUT "$API/source/snapshot" \
  -H "Content-Type: application/json" -H "Authorization: Bearer $SOURCE_TOKEN" -T "$DATA_DIR/data.json")
T1=$(now)
note "upload_seconds=$(elapsed "$T0" "$T1") upload_http=$CODE"
[ "$CODE" = 200 ] || { echo "::error::загрузка снимка: HTTP $CODE: $(head -c 300 "$OUT/upload.json")" >&2; exit 1; }
metrics | grep -E '^churn_snapshot_ingest_(peak_heap_bytes|stage_seconds)' >"$OUT/ingest-metrics.txt" || true

TOKEN=$(curl -fsS --max-time 60 -X POST "$API/auth/login" -H 'Content-Type: application/json' \
  -d "{\"login\":\"$LOGIN\",\"password\":\"$ADMIN_PASSWORD\"}" | jq -r .token)
echo "::add-mask::$TOKEN"
AUTH="Authorization: Bearer $TOKEN"
curl -fsS --max-time 120 "$API/customers?limit=1" -H "$AUTH" | jq '{total}' >"$OUT/customers-total.json" || true

# 3. Полный пересчёт: смена горизонта меняет настройки модели. PATCH отвечает сразу, пересчёт идёт после ответа,
# поэтому ждём, пока churn_model_version вырастет, и берём длительность и этапы из метрик этого пересчёта.
note "Пересчёт"
for H in 45 90 45; do
  V0=$(metrics | metric churn_model_version || true)
  T0=$(now)
  curl -fsS --max-time 1800 -X PATCH "$API/model/settings" -H "$AUTH" -H 'Content-Type: application/json' \
    -d "{\"horizonDays\":$H}" -o /dev/null
  T1=$(now)
  M=$(metrics); V1=$(metric churn_model_version <<<"$M")
  DEADLINE=$((SECONDS + 1800))
  while [ "${V1%.*}" = "${V0%.*}" ] && [ $SECONDS -lt $DEADLINE ]; do sleep 1; M=$(metrics); V1=$(metric churn_model_version <<<"$M"); done
  T2=$(now)
  note "recalc_horizon=$H version=${V0:-?}->${V1:-?} patch_seconds=$(elapsed "$T0" "$T1") until_new_version_seconds=$(elapsed "$T0" "$T2") server_recalc_seconds=$(metric churn_recalc_duration_seconds <<<"$M") recalc_customers=$(metric churn_recalc_customers <<<"$M")"
  grep -E '^churn_recalc_stage_seconds' <<<"$M" | tr '\n' ' ' | tee -a "$OUT/recalc-stages.txt"; echo | tee -a "$OUT/recalc-stages.txt"
done

# 4. Чтение очереди: 10 VU (k6/read.js, 30 с). Порог k6 не гейт — число идёт в отчёт.
if command -v k6 >/dev/null; then
  note "Чтение очереди"
  k6 run --quiet --summary-export "$OUT/k6-read.json" -e PASSWORD="$ADMIN_PASSWORD" -e LOGIN="$LOGIN" -e BASE_URL="$BASE_URL" \
    "$HERE/k6/read.js" >"$OUT/k6-read.log" 2>&1 || note "k6 read: пороги не выполнены (см. k6-read.log)"
fi

# 5. Залп 10 000 событий без читателей и SSE: скорость приёма на большом портфеле.
note "Залп 10 000"
BURST_NO_K6=1 OUT="$OUT/burst" SNAPSHOT= SOURCE_TOKEN= "$HERE/burst.sh" 10000 || note "burst: проверки залпа не прошли (см. burst/)"

# 6. Итог.
for f in "$OUT/burst/stream-10000.json" "$OUT/ingest-metrics.txt" "$OUT/customers-total.json"; do
  [ ! -f "$f" ] || { echo "--- $f"; cat "$f"; }
done
echo "--- k6 read"; jq -c '{p95: .metrics.http_req_duration["p(95)"], avg: .metrics.http_req_duration.avg, max: .metrics.http_req_duration.max, rps: .metrics.http_reqs.rate, failed: .metrics.http_req_failed.value}' "$OUT/k6-read.json" 2>/dev/null || true
kill "$SAMPLER" 2>/dev/null || true; wait "$SAMPLER" 2>/dev/null || true; trap - EXIT
PEAK_APP_MB=$(awk -F'\t' '$2+0>m {m=$2+0} END {printf "%.0f", m/1048576}' "$OUT/samples.tsv")
DB_BYTES=""
if command -v docker >/dev/null && [ -n "${DB_CONTAINER:-}" ]; then
  DB_BYTES=$(docker exec "$DB_CONTAINER" sh -c 'psql -U "${POSTGRES_USER:-postgres}" -At -c "select pg_database_size(current_database())"' 2>/dev/null || true)
fi
PEAK_DB=$(awk -F'\t' '{n=split($3, a, ";"); for (i=1;i<=n;i++) if (a[i] ~ /postgres/) { split(a[i], b, " "); print b[2] }}' "$OUT/samples.tsv" | sort -h | tail -1)
READ_P95=$(jq -r '.metrics.http_req_duration["p(95)"] // "н/д"' "$OUT/k6-read.json" 2>/dev/null || echo "н/д")
READ_FAIL=$(jq -r '.metrics.http_req_failed.value // "н/д"' "$OUT/k6-read.json" 2>/dev/null || echo "н/д")
{
  echo "## Миллион клиентов ($ACCOUNTS)"
  echo '```'
  cat "$OUT/steps.txt"
  echo "peak_app_rss_mb=$PEAK_APP_MB peak_db_mem=${PEAK_DB:-н/д} db_bytes=${DB_BYTES:-н/д} queue_read_p95_ms=$READ_P95 read_failed_rate=$READ_FAIL"
  echo '```'
  [ ! -f "$OUT/burst/summary.md" ] || cat "$OUT/burst/summary.md"
} | tee "$OUT/summary.md"
[ -z "${GITHUB_STEP_SUMMARY:-}" ] || cat "$OUT/summary.md" >>"$GITHUB_STEP_SUMMARY"
