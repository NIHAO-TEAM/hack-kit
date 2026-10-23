#!/usr/bin/env bash
# Сторож исполнителей: перезапускает routine, если сессия исполнителя, похоже, умерла. Без него PR висел бы
# до утра, пока человек не снимет и не поставит go:<id> снова.
#
# Смотрит открытые PR с go:<id>, без needs-human. Исполнитель должен ответить, а не отвечает:
#   - итог (бот: «[ci] итог ревью принят …» или «[review] … source: ci») новее его последнего [fix]/[merge]
#     и старше OWED_MIN минут;
#   - черновик с [start], а ни коммитов, ни его комментариев дольше DRAFT_MIN минут;
#   - метка стоит дольше START_MIN минут, а [start] так и не появился (запуск потерялся);
#   - needs-human сняли после его последнего ответа, а он не вернулся за UNHOLD_MIN минут;
#   - напоминание conflicts.sh «[guard] <id>: снова конфликт …» без ответа REMIND_MIN минут.
# Тогда комментарий «[guard] перезапуск исполнителя <id> (сторож) …» и /fire: новая сессия перехватывает PR
# по шагу 0 executor.md.
#
# Передача (решение владельца 24.10: ночью работа не встаёт из-за лимитов одного аккаунта): если исполнитель
# в team.md «Доступен: нет», или после перезапуска за START_MIN минут от него ни одного комментария (routine
# не стартует — кончились лимиты), или перезапусков уже MAX_RESTARTS, — сторож меняет go:<id> на go:<другой>
# (самый свободный доступный с местом по team.md), пишет [assign] и [guard] перезапуск и запускает routine
# нового. Некому передать — needs-human, как раньше.
#
# Ревьюер перезапускается каждые REVIEW_OWED_MIN минут до REVIEW_MAX_RESTARTS раз: лимиты аккаунта ревьюера
# обновляются через часы, и needs-human после трёх попыток останавливал бы PR до утра.
#
# Будит исполнителя только через wake-exec.sh (WAKE_FORCE=1: сторож сам установил, что сессия умерла).
#
# Дубли сессий: каждая сессия подписывает свои комментарии строкой «сессия: #xxxx» (executor.md). Если после
# последнего [start] исполнителя пишет ещё и сессия с другой подписью, старая не ушла — комментарий
# «[guard] дубль сессий <id>: …» (один раз на пару подписей), человеку — остановить лишнюю.
#
# Окружение: GH_TOKEN, GITHUB_REPOSITORY, RUNNER_TEMP, FIRE_<ID>_URL / FIRE_<ID>_TOKEN (по именам из watchdog.yml).
set -euo pipefail

REPO=${GITHUB_REPOSITORY:?}
OWED_MIN=40
VERDICT_MIN=20
DRAFT_MIN=45
START_MIN=30
UNHOLD_MIN=2
REMIND_MIN=20
MAX_RESTARTS=3
REVIEW_OWED_MIN=20
REVIEW_MAX_RESTARTS=30
now=$(date -u +%s)

age_min() { echo $(( (now - $(date -u -d "$1" +%s)) / 60 )); }

# Исполнители из team.md на main: id|доступен|лимит (разбор как в assign.sh). Checkout у сторожа частичный,
# поэтому файл — через API.
mapfile -t EXECS < <(gh api "repos/$REPO/contents/team.md" --jq .content | base64 -d | awk -F'|' '/^\| `exec-/ {
  id=$2; lim=$6; av=tolower($7); gsub(/[` ]/, "", id); gsub(/ /, "", lim); gsub(/ /, "", av)
  print id "|" av "|" lim }')
load_now() {
  LOAD=$(gh pr list -R "$REPO" --state open --limit 300 --json labels \
    --jq '[.[] | select([.labels[].name] | index("needs-human") | not) | .labels[].name | select(startswith("go:")) | ltrimstr("go:")]')
}
load_now
available() {
  local e i av lim
  for e in "${EXECS[@]}"; do IFS='|' read -r i av lim <<<"$e"; [ "$i" = "$1" ] && [ "$av" = "да" ] && return 0; done
  return 1
}
# Самый свободный доступный исполнитель с местом, кроме $1.
pick() {
  local best="" bestn=999 e i av lim n
  for e in "${EXECS[@]}"; do
    IFS='|' read -r i av lim <<<"$e"
    if [ "$i" = "$1" ] || [ "$av" != "да" ]; then continue; fi
    n=$(jq --arg i "$i" '[.[] | select(. == $i)] | length' <<<"$LOAD")
    if [ "$n" -lt "${lim:-1}" ] && [ "$n" -lt "$bestn" ]; then best=$i; bestn=$n; fi
  done
  printf '%s' "$best"
}
fire_exec() { # pr id причина; комментарий «[guard] перезапуск …» сторож пишет сам
  WAKE_FORCE=1 WAKE_COMMENT=0 bash .github/scripts/wake-exec.sh "$1" "$2" перезапуск "$3"
}
# Передача PR $1 от $2 другому исполнителю, причина $3. Метку go:* меняет сторож (исключение в AGENTS.md);
# [assign] нужен для проверки метки от бота в шаге 0 executor.md, [guard] перезапуск — чтобы новая сессия
# перехватила PR с первой неотмеченной задачи, а не начинала заново.
handover() {
  local pr=$1 from=$2 why=$3 to
  to=$(pick "$from")
  [ -n "$to" ] || return 1
  gh pr edit "$pr" -R "$REPO" --remove-label "go:$from" --add-label "go:$to"
  gh pr comment "$pr" -R "$REPO" --body "[assign] $to: передача от $from ($why). Изменение входит в утверждённый этап docs/plan.md; исполнитель перепроверит план."
  gh pr comment "$pr" -R "$REPO" --body "[guard] перезапуск исполнителя $to (сторож): передача от $from — $why. Прошлая сессия считается завершённой, новая перехватывает PR."
  load_now
  if fire_exec "$pr" "$to" "сторож: передача от $from — $why"; then
    restarted=$((restarted + 1)); echo "#$pr -> передача $from -> $to: $why"
  else
    gh label create needs-human --color B60205 -R "$REPO" >/dev/null 2>&1 || true
    gh pr edit "$pr" -R "$REPO" --add-label needs-human || true
    gh pr comment "$pr" -R "$REPO" --body "[guard] сторож не смог запустить $to через API-триггер. Нужен человек."
  fi
}

mapfile -t PRS < <(gh pr list -R "$REPO" --state open --limit 300 --json number,isDraft,labels \
  --jq '.[] | select([.labels[].name] | index("needs-human") | not)
        | ([.labels[].name | select(startswith("go:"))] | first) as $go | select($go != null)
        | "\(.number)|\(.isDraft)|\($go | ltrimstr("go:"))"')

restarted=0
for p in "${PRS[@]}"; do
  IFS='|' read -r pr draft id <<<"$p"
  comments=$(gh api "repos/$REPO/issues/$pr/comments" --paginate --jq '.[] | {u: .user.login, t: .created_at, b: .body}' | jq -s .)

  # Дубль: после последнего [start] пишет сессия с другой подписью, чем у этого [start].
  dup=$(jq -r '
    def tag: (.b | capture("сессия: #(?<s>[0-9a-f]{4})") | .s) // "";
    [.[] | select(.u != "github-actions[bot]")] as $ex
    | ([$ex[] | select(.b | startswith("[start]"))] | last) as $st
    | if $st == null or ($st | tag) == "" then "" else
        ($st | tag) as $cur
        | [$ex[] | select(.t > $st.t) | tag | select(. != "" and . != $cur)] | unique
        | if length == 0 then "" else "#\($cur) " + (map("#" + .) | join(" ")) end
      end' <<<"$comments")
  if [ -n "$dup" ]; then
    marker="[guard] дубль сессий $id: $dup"
    if [ "$(jq --arg m "$marker" '[.[] | select(.u == "github-actions[bot]" and (.b | startswith($m)))] | length' <<<"$comments")" -eq 0 ]; then
      gh pr comment "$pr" -R "$REPO" --body "$marker. PR ведёт сессия из последнего [start] (первая подпись); остальные должны уйти сами (executor.md, «Ожидание»). Не ушли — человеку исполнителя остановить их в claude.ai."
      echo "#$pr: дубль сессий $id: $dup"
    fi
  fi
  last() { jq -r --arg re "$1" --arg bot "$2" '[.[] | select((.b | test($re)) and ((.u == "github-actions[bot]") == ($bot == "1")))] | last | .t // ""' <<<"$comments"; }

  label_at=$(gh api "repos/$REPO/issues/$pr/events" --paginate \
    --jq "[.[] | select(.event == \"labeled\" and .label.name == \"go:$id\")] | last | .created_at // \"\"" | tail -n1)
  start_at=$(last "^\\[start\\] $id" 0)
  restart_at=$(last "^\\[guard\\] перезапуск исполнителя $id" 1)
  verdict_at=$(last '^(\[ci\] итог ревью принят|\[review\] [^\n]*\nhead: [^\n]*\nsource: ci)' 1)
  answer_at=$(last '^\[(fix|merge|needs-human)\]' 0)
  exec_at=$(last '^\[(start|fix|merge)\]' 0)
  commit_at=$(gh pr view "$pr" -R "$REPO" --json commits --jq '.commits | last | .committedDate // ""')

  if ! available "$id"; then
    handover "$pr" "$id" "в team.md исполнитель недоступен" || echo "#$pr: $id недоступен, передать некому"
    continue
  fi
  if [ -n "$restart_at" ] && [ "$(age_min "$restart_at")" -ge "$START_MIN" ]; then
    # Ответ — любой комментарий не бота с префиксом или коммит после перезапуска: [fix]/[merge] пишутся без id
    # (24.10 #66 ушёл от живого exec-2, когда искали только «[…] <id>»).
    alive=$(jq --arg t "$restart_at" '[.[] | select(.u != "github-actions[bot]" and .t > $t and (.b | startswith("[")) and (.b | test("^\\[(review|lead|guard|ci|assign)\\]") | not))] | length' <<<"$comments")
    [[ "${commit_at:-0}" > "$restart_at" ]] && alive=1
    if [ "$alive" -eq 0 ] && handover "$pr" "$id" "после перезапуска $(age_min "$restart_at") мин ни одного ответа — routine не стартует (лимиты аккаунта)"; then
      continue
    fi
  fi

  reason=""
  if [ -z "$start_at" ]; then
    [ -n "$label_at" ] && [ "$(age_min "$label_at")" -ge "$START_MIN" ] && reason="метка go:$id стоит $(age_min "$label_at") мин, а [start] нет"
  elif [ "$draft" = "true" ]; then
    latest=$(printf '%s\n%s\n%s\n' "$exec_at" "$commit_at" "$restart_at" | sort | tail -n1)
    [ "$(age_min "$latest")" -ge "$DRAFT_MIN" ] && reason="черновик без коммитов и комментариев исполнителя $(age_min "$latest") мин"
  elif [ -n "$verdict_at" ] && [[ "$verdict_at" > "${answer_at:-0}" ]]; then
    [ "$(age_min "$verdict_at")" -ge "$VERDICT_MIN" ] && reason="итог $(age_min "$verdict_at") мин без ответа исполнителя"
  fi
  # needs-human сняли после последнего ответа исполнителя: его сессия кончилась на [needs-human], комментарий
  # человека или пахана её не будит (23.10: #66 стоял больше часа после снятия метки).
  unhold_at=$(gh api "repos/$REPO/issues/$pr/events" --paginate \
    --jq '[.[] | select(.event == "unlabeled" and .label.name == "needs-human")] | last | .created_at // ""' | tail -n1)
  if [ -z "$reason" ] && [ -n "$unhold_at" ] && [[ "$unhold_at" > "${exec_at:-0}" ]] && [[ "$unhold_at" > "${restart_at:-0}" ]] \
     && [ "$(age_min "$unhold_at")" -ge "$UNHOLD_MIN" ]; then
    reason="needs-human снят $(age_min "$unhold_at") мин назад, исполнитель не вернулся"
  fi
  # Напоминание о конфликте (conflicts.sh, сессия казалась живой) осталось без ответа: сессия умерла.
  remind_at=$(jq -r --arg id "$id" '[.[] | select(.u == "github-actions[bot]" and (.b | startswith("[guard] " + $id + ": ")))] | last | .t // ""' <<<"$comments")
  if [ -z "$reason" ] && [ -n "$remind_at" ] && [[ "$remind_at" > "${exec_at:-0}" ]] && [[ "$remind_at" > "${commit_at:-0}" ]]      && [[ "$remind_at" > "${restart_at:-0}" ]] && [ "$(age_min "$remind_at")" -ge "$REMIND_MIN" ]; then
    reason="напоминание о конфликте с main $(age_min "$remind_at") мин без ответа"
  fi
  [ -n "$reason" ] || continue
  # Перезапуск уже был после последнего события, которого ждём, — новой сессии нужно время.
  if [ -n "$restart_at" ] && [ "$(age_min "$restart_at")" -lt "$OWED_MIN" ]; then continue; fi

  n=$(jq --arg id "$id" '[.[] | select(.u == "github-actions[bot]" and (.b | startswith("[guard] перезапуск исполнителя " + $id + " (сторож)")))] | length' <<<"$comments")
  if [ "$n" -ge "$MAX_RESTARTS" ]; then
    handover "$pr" "$id" "сторож перезапускал $id $n раз, а PR стоит ($reason)" && continue
    gh label create needs-human --color B60205 -R "$REPO" >/dev/null 2>&1 || true
    gh pr edit "$pr" -R "$REPO" --add-label needs-human
    gh pr comment "$pr" -R "$REPO" --body "[guard] сторож перезапускал $id уже $n раз, а PR стоит ($reason), передать некому. Нужен человек."
    continue
  fi

  gh pr comment "$pr" -R "$REPO" --body "[guard] перезапуск исполнителя $id (сторож): $reason. Прошлая сессия считается завершённой, новая перехватывает PR."
  if fire_exec "$pr" "$id" "сторож: $reason"; then
    restarted=$((restarted + 1)); echo "#$pr -> перезапуск $id: $reason"
  else
    gh label create needs-human --color B60205 -R "$REPO" >/dev/null 2>&1 || true
    gh pr edit "$pr" -R "$REPO" --add-label needs-human || true
    gh pr comment "$pr" -R "$REPO" --body "[guard] сторож не смог запустить $id через API-триггер (секреты FIRE_$(printf "%s" "$id" | tr "a-z-" "A-Z_")_*). Нужен человек."
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
  [ "$(age_min "$since")" -ge "$REVIEW_OWED_MIN" ] || continue
  n=$(jq --arg h "$head" '[.[] | select(.u == "github-actions[bot]" and (.b | startswith("[guard] перезапуск ревьюера")) and (.b | contains($h)))] | length' <<<"$comments")
  if [ "$n" -ge "$REVIEW_MAX_RESTARTS" ]; then
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
