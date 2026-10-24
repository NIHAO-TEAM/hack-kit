#!/usr/bin/env bash
# Сверка Go с эталоном на сгенерированном снимке: parity.sh N [SEED]
# snapgen -> snapcalc (Go) и oracle.py -> parity_compare.py. Код 1 при расхождении.
# Рабочие файлы — во временной папке (PARITY_DIR, иначе mktemp), снимок на 300k занимает сотни МБ.
set -euo pipefail

N=${1:?usage: parity.sh N [SEED]}
SEED=${2:-1}
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
DIR=${PARITY_DIR:-$(mktemp -d)}
mkdir -p "$DIR"
cd "$ROOT/backend"
go build -o "$DIR/snapgen" ./tools/snapgen/cmd
go build -o "$DIR/snapcalc" ./tools/snapcalc/cmd

now() { date +%s.%N; }
elapsed() { awk -v a="$1" -v b="$2" 'BEGIN { printf "%.1f", b - a }'; }

"$DIR/snapgen" -accounts "$N" -seed "$SEED" > "$DIR/snap.json"
ASOF=$(python3 -I -c 'import json,sys; print(json.load(open(sys.argv[1]))["asOf"])' "$DIR/snap.json")

t0=$(now); "$DIR/snapcalc" -in "$DIR/snap.json" -as-of "$ASOF" > "$DIR/go.jsonl"; t1=$(now)
python3 -I "$ROOT/tools/oracle/oracle.py" "$DIR/snap.json" --as-of "$ASOF" --per-customer > "$DIR/oracle.jsonl"; t2=$(now)
echo "N=$N seed=$SEED asOf=$ASOF время Go: $(elapsed "$t0" "$t1") с, эталон: $(elapsed "$t1" "$t2") с"
python3 -I "$ROOT/tools/oracle/parity_compare.py" "$DIR/go.jsonl" "$DIR/oracle.jsonl"
