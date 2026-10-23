#!/usr/bin/env bash
# Автоназначение: запускает исполнителей на изменения утверждённых этапов.
#
# Утверждение этапа — это строка в docs/plan.md со статусом «утверждён» или «в работе»; статус ставит пахан
# по «да» человека на запуске хака или после сессии эксперта (lead.md). Скрипт берёт из main:
#   - docs/plan.md: какие изменения входят в утверждённые этапы;
#   - team.md: исполнители, их части, «Доступен» и лимит «Изменений на руках».
# Кандидат — черновик PR с меткой change и веткой change/<имя>, ещё без go:* и needs-human. Берутся по
# порядку создания, пока у подходящего исполнителя есть место. Остальные ждут: скрипт вызывается снова,
# когда PR закрыт, план или team.md изменены, открыт новый черновик.
#
# Изменение с «!» перед именем в строке этапа (`!orders-contract`) блокирующее: контракт, миграции, общие
# сценарии. Пока оно не слито (нет папки в changes/archive/ на main), остальные изменения этапа не
# назначаются: параллельные PR иначе правят один openapi.yaml и ловят конфликты друг за другом.
#
# «этап N» в столбце «Зависимости» — этап ждёт, пока слиты все изменения этапа N. Так весь план утверждается
# на запуске, а этапы идут друг за другом сами, без человека (ночью тоже).
#
# Метку go:<id> ставит бот. События от GITHUB_TOKEN другие workflow не запускают, поэтому guard.yml
# здесь не срабатывает, исполнителя будит сам скрипт (fire.sh). Исполнитель при этом сам перепроверяет
# план в main (executor.md, шаг 0).
#
# Окружение: GH_TOKEN, GITHUB_REPOSITORY, RUNNER_TEMP, FIRE_<ID>_URL / FIRE_<ID>_TOKEN исполнителей
# (assign.yml передаёт их по именам, в лог не печатать).
set -euo pipefail

REPO=${GITHUB_REPOSITORY:?}
PLAN=docs/plan.md
TEAM=team.md
# Строки этапов ищутся только в разделе «Разбивка на этапы»: в других таблицах плана первый столбец
# тоже бывает числом (23.10 строка «| 2 |» входных данных совпала с этапом 2, блокировка «!» не сработала).
STAGES=$(awk '/^## Разбивка на этапы/{f=1; next} /^## /{f=0} f' "$PLAN")
SHA=$(git rev-parse --short HEAD)

# Исполнители: id|части|доступен|лимит
mapfile -t EXECS < <(awk -F'|' '/^\| `exec-/ {
  id=$2; parts=tolower($4); lim=$6; av=tolower($7)
  gsub(/[` ]/, "", id); gsub(/ /, "", parts); gsub(/ /, "", lim); gsub(/ /, "", av)
  print id "|" parts "|" av "|" lim }' "$TEAM")
[ "${#EXECS[@]}" -gt 0 ] || { echo "в team.md нет исполнителей"; exit 0; }

# Номер этапа, если изменение $1 (или его имя без даты $2) входит в утверждённый этап.
stage_of() {
  local name=$1 short=$2 line st
  while IFS= read -r line; do
    st=$(awk -F'|' '{s=$(NF-1); gsub(/^ +| +$/, "", s); print s}' <<<"$line")
    [ "$st" = "утверждён" ] || [ "$st" = "в работе" ] || continue
    if grep -Eq "(^|[^A-Za-z0-9-])(${name}|${short})([^A-Za-z0-9-]|$)" <<<"$line"; then
      awk -F'|' '{n=$2; gsub(/ /, "", n); print n}' <<<"$line"
      return 0
    fi
  done < <(grep '^|' <<<"$STAGES" || true)
  return 1
}

# Слитые изменения: папки changes/archive/ на main (архив делается в том же PR, что и код).
ARCHIVE=$(gh api "repos/$REPO/contents/changes/archive" --jq '.[].name' 2>/dev/null || true)
archived() {
  local short
  short=$(sed -E 's/^[0-9]{4}-[0-9]{2}-[0-9]{2}-//' <<<"$1")
  grep -Eqx "([0-9]{4}-[0-9]{2}-[0-9]{2}-)?${short}" <<<"$ARCHIVE"
}

# Первое неслитое блокирующее изменение («!имя») этапа $1, стоящее в строке раньше кандидата ($2 полное имя,
# $3 без даты). Несколько «!» идут по порядку строки: каждое ждёт только предыдущие, взаимной блокировки нет.
blocked_by() {
  local line b
  line=$(grep -E "^\| *$1 *\|" <<<"$STAGES" | head -n1)
  for b in $(grep -oE '!`?[0-9a-z-]+' <<<"$line" | tr -d '!`'); do
    if [ "$b" = "$2" ] || [ "$b" = "$3" ]; then return 1; fi
    if archived "$b"; then continue; fi
    printf '%s' "$b"
    return 0
  done
  return 1
}

# Первый этап из «Зависимостей» этапа $1 (столбец 7 таблицы, «этап N»), у которого слиты не все изменения
# (столбец 6). Пустая строка изменений — этап без кода (документы пахана), считается выполненным.
waits_for_stage() {
  local deps d line c
  deps=$(grep -E "^\| *$1 *\|" <<<"$STAGES" | head -n1 | awk -F'|' '{print $8}')
  for d in $(grep -oE 'этап +[0-9]+' <<<"$deps" | grep -oE '[0-9]+'); do
    line=$(grep -E "^\| *$d *\|" <<<"$STAGES" | head -n1)
    for c in $(awk -F'|' '{print $7}' <<<"$line" | grep -oE '`!?[0-9a-z-]+`' | tr -d '`!'); do
      archived "$c" || { printf '%s' "$d"; return 0; }
    done
  done
  return 1
}

# Части, которые задевает изменение (по proposal.md в ветке PR): backend, frontend. Пусто — любая.
parts_of() {
  local branch=$1 text
  text=$(gh api "repos/$REPO/contents/changes/${branch#change/}/proposal.md?ref=$branch" \
    -H "Accept: application/vnd.github.raw" 2>/dev/null || true)
  for p in backend frontend; do
    grep -Eiq "^- *${p}: *[^н]" <<<"$text" && printf '%s ' "$p"
  done
  true
}

# Открытые PR — одним обычным запросом. Фильтры --draft и --label у «gh pr list» идут через поисковый API,
# а его индекс отстаёт: 22.10 запуск сразу после смены меток получил пустой список и пропустил кандидата.
OPEN_PRS=$(gh pr list -R "$REPO" --state open --limit 300 --json number,isDraft,headRefName,labels,createdAt)

declare -A LOAD
load_of() {
  local id=$1
  if [ -z "${LOAD[$id]:-}" ]; then
    # PR с needs-human стоит до человека и место исполнителя не занимает: иначе ночью два застрявших PR
    # останавливали бы исполнителя целиком.
    LOAD[$id]=$(jq --arg l "go:$id" '[.[] | select(([.labels[].name] | index($l)) and ([.labels[].name] | index("needs-human") | not))] | length' <<<"$OPEN_PRS")
  fi
  printf '%s' "${LOAD[$id]}"
}

fire_executor() { # id pr stage
  local id=$1 pr=$2 stage=$3 sid
  sid=$(printf '%s' "$id" | tr 'a-z-' 'A-Z_')
  local url_var="FIRE_${sid}_URL" token_var="FIRE_${sid}_TOKEN"
  FIRE_URL=${!url_var:-}
  FIRE_TOKEN=${!token_var:-}
  export FIRE_URL FIRE_TOKEN
  bash .github/scripts/fire.sh "PR #$pr в $REPO, исполнитель $id, старт (автоназначение по этапу $stage)"
}

mapfile -t CANDS < <(jq -r 'sort_by(.createdAt) | .[]
        | select(.isDraft and ([.labels[].name] | index("change")))
        | select(.headRefName | startswith("change/"))
        | select([.labels[].name | select(startswith("go:") or . == "needs-human")] | length == 0)
        | "\(.number)|\(.headRefName)"' <<<"$OPEN_PRS")

assigned=0
for c in "${CANDS[@]}"; do
  pr=${c%%|*}; branch=${c#*|}
  name=${branch#change/}
  short=$(sed -E 's/^[0-9]{4}-[0-9]{2}-[0-9]{2}-//' <<<"$name")
  stage=$(stage_of "$name" "$short") || { echo "#$pr $name: нет в утверждённом этапе — жду метку go: от человека"; continue; }
  if dep=$(waits_for_stage "$stage"); then
    echo "#$pr $name (этап $stage): ждёт слияния всех изменений этапа $dep — в очереди"; continue
  fi
  if blocker=$(blocked_by "$stage" "$name" "$short"); then
    echo "#$pr $name (этап $stage): ждёт слияния блокирующего $blocker — в очереди"; continue
  fi

  need=$(parts_of "$branch")
  # Наименее загруженный подходящий исполнитель, при равной нагрузке — первый по team.md. «Первый со
  # свободным местом» складывал бы этап на exec-1 и exec-2, пока остальные простаивают.
  picked=""; best=""
  for e in "${EXECS[@]}"; do
    IFS='|' read -r id parts av lim <<<"$e"
    [ "$av" = "да" ] || continue
    ok=1
    for p in $need; do case "$parts" in *"$p"*) ;; *) ok=0 ;; esac; done
    [ "$ok" = 1 ] || continue
    load=$(load_of "$id")
    [ "$load" -lt "$lim" ] || continue
    if [ -z "$best" ] || [ "$load" -lt "$best" ]; then picked=$id; best=$load; fi
  done
  [ -n "$picked" ] || { echo "#$pr $name (этап $stage): нет свободного исполнителя — в очереди"; continue; }

  gh pr comment "$pr" -R "$REPO" --body "[assign] $picked: изменение входит в этап $stage, утверждённый в docs/plan.md (main@$SHA). Запускаю исполнителя без метки от человека; исполнитель перепроверит план сам."
  gh label create "go:$picked" -R "$REPO" --color 0E8A16 >/dev/null 2>&1 || true
  if fire_executor "$picked" "$pr" "$stage"; then
    gh pr edit "$pr" -R "$REPO" --add-label "go:$picked"
    LOAD[$picked]=$(( $(load_of "$picked") + 1 ))
    assigned=$((assigned + 1))
    echo "#$pr -> $picked (этап $stage)"
  else
    gh label create needs-human --color B60205 -R "$REPO" >/dev/null 2>&1 || true
    gh pr edit "$pr" -R "$REPO" --add-label needs-human || true
    gh pr comment "$pr" -R "$REPO" --body "[assign] не удалось запустить $picked через API-триггер (секреты FIRE_*). Нужен человек."
  fi
done
echo "назначено: $assigned"
