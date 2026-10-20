#!/usr/bin/env sh
# Создаёт метки процесса в репозитории (один раз в день 0). Нужен gh с правами на репозиторий.
# Использование: scripts/labels.sh exec-1 exec-2 ...   (id исполнителей из team.md)
set -eu

mk() { gh label create "$1" --color "$2" --description "$3" --force >/dev/null && echo "метка: $1"; }

mk change       1D76DB "Изменение из changes/"
mk critic       5319E7 "Отчёт критика"
mk risk:low     C2E0C6 "Быстрая полоса ревью: тексты, документация"
mk protected    B60205 "Трогает защищённые пути: внимание ревьюеру"
mk manual-merge B60205 "Трогает правила процесса: сливает человек"
mk needs-human  B60205 "Нужен человек"
for n in 1 2 3 4; do mk "round-$n" FBCA04 "Раунд ревью $n"; done

for id in "$@"; do
  mk "go:$id" 0E8A16 "Согласовано человеком, исполнитель $id"
done
