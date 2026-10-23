#!/usr/bin/env bash
# Сторож исполнителей: перезапускает routine, если сессия исполнителя, похоже, умерла. Без него PR висел бы
# до утра, пока человек не снимет и не поставит go:<id> снова.
#
# Смотрит открытые PR с go:<id>, без needs-human. Исполнитель должен ответить, а не отвечает:
#   - итог (бот: «[ci] итог ревью принят …» или «[review] … source: ci») новее его последнего [fix]/[merge]
#     и старше OWED_MIN минут;
#   - черновик с [start], а ни коммитов, ни его комментариев дольше DRAFT_MIN минут;
#   - метка стоит дольше START_MIN минут, а [start] так и не появился (запуск потерялся).
# Тогда комментарий «[guard] перезапуск исполнителя <id> (сторож) …» и /fire: новая сессия перехватывает PR
# по шагу 0 executor.md. После MAX_RESTARTS перезапусков сторожа на PR — needs-human: значит, дело не
# в обрыве сессии.
#
# Окружение: GH_TOKEN, GITHUB_REPOSITORY, RUNNER_TEMP, FIRE_<ID>_URL / FIRE_<ID>_TOKEN (по именам из watchdog.yml).
set -euo pipefail

REPO=${GITHUB_REPOSITORY:?}
OWED_MIN=40
DRAFT_MIN=45
START_MIN=30
MAX_RESTARTS=3
now=$(date -u +%s)

age_min() { echo $(( (now - $(date -u -d "$1" +%s)) / 60 )); }

mapfile -t PRS < <(gh pr list -R "$REPO" --state open --limit 300 --json number,isDraft,labels \
  --jq '.[] | select([.labels[].name] | index("needs-human") | not)
        | ([.labels[].name | select(startswith("go:"))] | first) as $go | select($go != null)
        | "\(.number)|\(.isDraft)|\($go | ltrimstr("go:"))"')

restarted=0
for p in "${PRS[@]}"; do
  IFS='|' read -r pr draft id <<<"$p"
  comments=$(gh api "repos/$REPO/issues/$pr/comments" --paginate --jq '.[] | {u: .user.login, t: .created_at, b: .body}' | jq -s .)
  last() { jq -r --arg re "$1" --arg bot "$2" '[.[] | select((.b | test($re)) and ((.u == "github-actions[bot]") == ($bot == "1")))] | last | .t // ""' <<<"$comments"; }

  label_at=$(gh api "repos/$REPO/issues/$pr/events" --paginate \
    --jq "[.[] | select(.event == \"labeled\" and .label.name == \"go:$id\")] | last | .created_at // \"\"" | tail -n1)
  start_at=$(last "^\\[start\\] $id" 0)
  restart_at=$(last "^\\[guard\\] перезапуск исполнителя $id" 1)
  verdict_at=$(last '^(\[ci\] итог ревью принят|\[review\] [^\n]*\nhead: [^\n]*\nsource: ci)' 1)
  answer_at=$(last '^\[(fix|merge|needs-human)\]' 0)
  exec_at=$(last '^\[(start|fix|merge)\]' 0)
  commit_at=$(gh pr view "$pr" -R "$REPO" --json commits --jq '.commits | last | .committedDate // ""')

  reason=""
  if [ -z "$start_at" ]; then
    [ -n "$label_at" ] && [ "$(age_min "$label_at")" -ge "$START_MIN" ] && reason="метка go:$id стоит $(age_min "$label_at") мин, а [start] нет"
  elif [ "$draft" = "true" ]; then
    latest=$(printf '%s\n%s\n%s\n' "$exec_at" "$commit_at" "$restart_at" | sort | tail -n1)
    [ "$(age_min "$latest")" -ge "$DRAFT_MIN" ] && reason="черновик без коммитов и комментариев исполнителя $(age_min "$latest") мин"
  elif [ -n "$verdict_at" ] && [[ "$verdict_at" > "${answer_at:-0}" ]]; then
    [ "$(age_min "$verdict_at")" -ge "$OWED_MIN" ] && reason="итог $(age_min "$verdict_at") мин без ответа исполнителя"
  fi
  [ -n "$reason" ] || continue
  # Перезапуск уже был после последнего события, которого ждём, — новой сессии нужно время.
  if [ -n "$restart_at" ] && [ "$(age_min "$restart_at")" -lt "$OWED_MIN" ]; then continue; fi

  n=$(jq --arg id "$id" '[.[] | select(.u == "github-actions[bot]" and (.b | startswith("[guard] перезапуск исполнителя " + $id + " (сторож)")))] | length' <<<"$comments")
  if [ "$n" -ge "$MAX_RESTARTS" ]; then
    gh label create needs-human --color B60205 -R "$REPO" >/dev/null 2>&1 || true
    gh pr edit "$pr" -R "$REPO" --add-label needs-human
    gh pr comment "$pr" -R "$REPO" --body "[guard] сторож перезапускал $id уже $n раз, а PR стоит ($reason). Дело не в обрыве сессии — нужен человек."
    continue
  fi

  sid=$(printf '%s' "$id" | tr 'a-z-' 'A-Z_')
  url_var="FIRE_${sid}_URL"; token_var="FIRE_${sid}_TOKEN"
  export FIRE_URL=${!url_var:-} FIRE_TOKEN=${!token_var:-}
  gh pr comment "$pr" -R "$REPO" --body "[guard] перезапуск исполнителя $id (сторож): $reason. Прошлая сессия считается завершённой, новая перехватывает PR."
  if bash .github/scripts/fire.sh "PR #$pr в $REPO, исполнитель $id, перезапуск (сторож: $reason)"; then
    restarted=$((restarted + 1)); echo "#$pr -> перезапуск $id: $reason"
  else
    gh label create needs-human --color B60205 -R "$REPO" >/dev/null 2>&1 || true
    gh pr edit "$pr" -R "$REPO" --add-label needs-human || true
    gh pr comment "$pr" -R "$REPO" --body "[guard] сторож не смог запустить $id через API-триггер (секреты FIRE_${sid}_*). Нужен человек."
  fi
done

# Ревьюер: CI запросил ревью («[ci] checks зелёные, ревью запрошено» или «[ci] оспаривание …»), а итога
# [review] на этот head нет дольше OWED_MIN минут — routine ревьюера не отработала. Перезапуск тем же текстом,
# что у ci.yml; после MAX_RESTARTS на один head — needs-human.
mapfile -t OPEN < <(gh pr list -R "$REPO" --state open --limit 300 --json number,isDraft,labels \
  --jq '.[] | select((.isDraft | not) and ([.labels[].name] | index("needs-human") | not)) | .number')
for pr in "${OPEN[@]}"; do
  comments=$(gh api "repos/$REPO/issues/$pr/comments" --paginate --jq '.[] | {u: .user.login, t: .created_at, b: .body}' | jq -s .)
  req=$(jq -c '[.[] | select(.u == "github-actions[bot]" and (.b | test("^\\[ci\\] (checks зелёные, ревью запрошено|оспаривание: ревью запрошено повторно)")))] | last // empty' <<<"$comments")
  [ -n "$req" ] || continue
  req_at=$(jq -r .t <<<"$req")
  n_round=$(jq -r '.b | capture("round: (?<n>[0-9]+)").n' <<<"$req")
  head=$(jq -r '.b | capture("head: (?<h>[0-9a-f]{40})").h' <<<"$req")
  [ "$(gh pr view "$pr" -R "$REPO" --json headRefOid --jq .headRefOid)" = "$head" ] || continue
  answered=$(jq --arg h "$head" --arg t "$req_at" '[.[] | select((.b | startswith("[review]")) and (.b | contains("head: " + $h)) and .t > $t)] | length' <<<"$comments")
  [ "$answered" -eq 0 ] || continue
  last_kick=$(jq -r --arg h "$head" '[.[] | select(.u == "github-actions[bot]" and (.b | startswith("[guard] перезапуск ревьюера")) and (.b | contains($h)))] | last | .t // ""' <<<"$comments")
  since=${last_kick:-$req_at}
  [ "$(age_min "$since")" -ge "$OWED_MIN" ] || continue
  n=$(jq --arg h "$head" '[.[] | select(.u == "github-actions[bot]" and (.b | startswith("[guard] перезапуск ревьюера")) and (.b | contains($h)))] | length' <<<"$comments")
  if [ "$n" -ge "$MAX_RESTARTS" ]; then
    gh label create needs-human --color B60205 -R "$REPO" >/dev/null 2>&1 || true
    gh pr edit "$pr" -R "$REPO" --add-label needs-human
    gh pr comment "$pr" -R "$REPO" --body "[guard] ревьюер не ответил на head $head после $n перезапусков. Нужен человек (лимиты или routine ревьюера)."
    continue
  fi
  export FIRE_URL=${FIRE_REVIEWER_URL:-} FIRE_TOKEN=${FIRE_REVIEWER_TOKEN:-}
  gh pr comment "$pr" -R "$REPO" --body "[guard] перезапуск ревьюера (сторож): итога на head $head нет $(age_min "$since") мин."
  if bash .github/scripts/fire.sh "PR #$pr в $REPO: ревью, round $n_round, head $head"; then
    restarted=$((restarted + 1)); echo "#$pr -> перезапуск ревьюера"
  else
    gh label create needs-human --color B60205 -R "$REPO" >/dev/null 2>&1 || true
    gh pr edit "$pr" -R "$REPO" --add-label needs-human || true
    gh pr comment "$pr" -R "$REPO" --body "[guard] сторож не смог запустить ревьюера через API-триггер (секреты FIRE_REVIEWER_*). Нужен человек."
  fi
done
echo "перезапущено: $restarted"
