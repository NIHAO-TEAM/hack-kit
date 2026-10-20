#!/usr/bin/env bash
# Автоназначение: запускает исполнителей на изменения утверждённых этапов.
#
# Утверждение этапа — это строка в docs/plan.md со статусом «утверждён» или «в работе»; файл меняет только
# человек (docs/plan.md в manual-merge-paths.txt). Скрипт берёт из main:
#   - docs/plan.md: какие изменения входят в утверждённые этапы;
#   - team.md: исполнители, их части, «Доступен» и лимит «Изменений на руках».
# Кандидат — черновик PR с меткой change и веткой change/<имя>, ещё без go:* и needs-human. Берутся по
# порядку создания, пока у подходящего исполнителя есть место. Остальные ждут: скрипт вызывается снова,
# когда PR закрыт, план или team.md изменены, открыт новый черновик.
#
# Метку go:<id> ставит бот. События от GITHUB_TOKEN другие workflow не запускают, поэтому guard.yml
# здесь не срабатывает, исполнителя будит сам скрипт (fire.sh). Исполнитель при этом сам перепроверяет
# план в main (executor.md, шаг 0).
#
# Окружение: GH_TOKEN, GITHUB_REPOSITORY, RUNNER_TEMP, SECRETS_JSON (toJSON(secrets): только чтение
# FIRE_<ID>_URL / FIRE_<ID>_TOKEN, в лог не печатать).
set -euo pipefail

REPO=${GITHUB_REPOSITORY:?}
PLAN=docs/plan.md
TEAM=team.md
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
  done < <(grep '^|' "$PLAN" || true)
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

declare -A LOAD
load_of() {
  local id=$1
  if [ -z "${LOAD[$id]:-}" ]; then
    LOAD[$id]=$(gh pr list -R "$REPO" --state open --label "go:$id" --json number --jq length)
  fi
  printf '%s' "${LOAD[$id]}"
}

fire_executor() { # id pr stage
  local id=$1 pr=$2 stage=$3 sid
  sid=$(printf '%s' "$id" | tr 'a-z-' 'A-Z_')
  FIRE_URL=$(jq -r --arg k "FIRE_${sid}_URL" '.[$k] // empty' <<<"$SECRETS_JSON")
  FIRE_TOKEN=$(jq -r --arg k "FIRE_${sid}_TOKEN" '.[$k] // empty' <<<"$SECRETS_JSON")
  export FIRE_URL FIRE_TOKEN
  bash .github/scripts/fire.sh "PR #$pr в $REPO, исполнитель $id, старт (автоназначение по этапу $stage)"
}

mapfile -t CANDS < <(gh pr list -R "$REPO" --state open --draft --label change \
  --json number,headRefName,labels,createdAt \
  --jq 'sort_by(.createdAt) | .[]
        | select(.headRefName | startswith("change/"))
        | select([.labels[].name | select(startswith("go:") or . == "needs-human")] | length == 0)
        | "\(.number)|\(.headRefName)"')

assigned=0
for c in "${CANDS[@]}"; do
  pr=${c%%|*}; branch=${c#*|}
  name=${branch#change/}
  short=$(sed -E 's/^[0-9]{4}-[0-9]{2}-[0-9]{2}-//' <<<"$name")
  stage=$(stage_of "$name" "$short") || { echo "#$pr $name: нет в утверждённом этапе — жду метку go: от человека"; continue; }

  need=$(parts_of "$branch")
  picked=""
  for e in "${EXECS[@]}"; do
    IFS='|' read -r id parts av lim <<<"$e"
    [ "$av" = "да" ] || continue
    ok=1
    for p in $need; do case "$parts" in *"$p"*) ;; *) ok=0 ;; esac; done
    [ "$ok" = 1 ] || continue
    [ "$(load_of "$id")" -lt "$lim" ] || continue
    picked=$id; break
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
