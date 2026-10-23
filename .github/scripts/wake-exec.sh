#!/usr/bin/env bash
# Единственный способ разбудить исполнителя (assign, guard, watchdog, conflicts, review-post). Решает, нужна ли
# новая сессия, чтобы на одном PR не работали две: 24.10 conflicts.sh и review-post.yml каждый по-своему решали,
# жива ли сессия, и поднимали 3–4 сессии на PR (#82, #89, #92, #95). Новый workflow будит исполнителя только
# через этот скрипт и своих проверок «жива ли сессия» не пишет (lead.md, «Автоматика»).
#
# Использование: wake-exec.sh <pr> <id> <старт|перезапуск> <причина>
#   Для «перезапуск» пишет «[guard] перезапуск исполнителя <id> (<причина>): …» (шаг 0 executor.md: перехват),
#   затем /fire. WAKE_COMMENT=0 — комментарий уже написал вызвавший.
#
# Аренда PR (без WAKE_FORCE=1):
#   - exit 11 «запуск в пути»: исполнителя будили меньше LEASE_MIN минут назад, а он ещё не отметился;
#   - exit 10 «сессия жива»: последний статус исполнителя не конечный (не «[wait] … нет подписки», не
#     [needs-human]) и всё, что ему адресовано после его последнего ответа, моложе LEASE_MIN минут. Подписанная
#     сессия получит событие сама; вызвавший может оставить напоминание «[guard] <id>: …» — оно тоже считается
#     адресованным, и без ответа за LEASE_MIN минут следующий вызов разбудит новую сессию.
# WAKE_FORCE=1 — осознанный запуск, аренду не смотрит: метка от человека (guard.yml), автоназначение нового PR
# (assign.sh), сторож, который сам установил, что сессия умерла (watchdog.sh).
#
# exit 0 — запущено, 1 — не удалось (секреты, API), 10/11 — не запускал.
# Окружение: GH_TOKEN, GITHUB_REPOSITORY, RUNNER_TEMP; FIRE_<ID>_URL / FIRE_<ID>_TOKEN или готовые FIRE_URL / FIRE_TOKEN.
set -euo pipefail

pr=${1:?pr} id=${2:?id} kind=${3:?старт|перезапуск} why=${4:?причина}
REPO=${GITHUB_REPOSITORY:?}
LEASE_MIN=${WAKE_LEASE_MIN:-20}
now=$(date -u +%s)
age_min() { echo $(( (now - $(date -u -d "$1" +%s)) / 60 )); }

if [ "${WAKE_FORCE:-0}" != 1 ]; then
  comments=$(gh api "repos/$REPO/issues/$pr/comments" --paginate --jq '.[] | {u: .user.login, t: .created_at, b: .body}' | jq -s .)
  commit_at=$(gh pr view "$pr" -R "$REPO" --json commits --jq '.commits | last | .committedDate // ""')
  # Ответ исполнителя — комментарий не бота с префиксом (кроме ревьюера и пахана) или коммит.
  last_exec=$(jq -c '[.[] | select(.u != "github-actions[bot]" and (.b | startswith("[")) and (.b | test("^\\[(review|lead|guard|ci|assign)\\]") | not))] | last // {}' <<<"$comments")
  exec_at=$(jq -r '.t // ""' <<<"$last_exec")
  exec_body=$(jq -r '.b // ""' <<<"$last_exec")
  [[ "${commit_at:-}" > "$exec_at" ]] && exec_at=$commit_at
  wake_at=$(jq -r --arg id "$id" '[.[] | select(.u == "github-actions[bot]" and ((.b | startswith("[guard] перезапуск исполнителя " + $id)) or (.b | startswith("[assign] " + $id))))] | last | .t // ""' <<<"$comments")

  if [ -n "$wake_at" ] && ! [[ "$exec_at" > "$wake_at" ]] && [ "$(age_min "$wake_at")" -lt "$LEASE_MIN" ]; then
    echo "wake: #$pr $id — разбужен $(age_min "$wake_at") мин назад, ещё не отметился, не запускаю"; exit 11
  fi
  final=0
  case "$exec_body" in "" | "[wait]"*"нет подписки"* | "[needs-human]"*) final=1 ;; esac
  if [ "$final" = 0 ]; then
    owed=$(jq -r --arg id "$id" --arg t "$exec_at" '[.[] | select(.u == "github-actions[bot]" and .t > $t
        and (.b | test("^(\\[guard\\] перезапуск исполнителя " + $id + "|\\[guard\\] " + $id + ": |\\[ci\\] итог ревью принят|\\[review\\] [^\\n]*\\nhead: [^\\n]*\\nsource: ci)")))]
        | first | .t // ""' <<<"$comments")
    if [ -z "$owed" ] || [ "$(age_min "$owed")" -lt "$LEASE_MIN" ]; then
      echo "wake: #$pr $id — сессия жива (ответ $(age_min "$exec_at") мин назад), не запускаю"; exit 10
    fi
  fi
fi

sid=$(printf '%s' "$id" | tr 'a-z-' 'A-Z_')
url_var="FIRE_${sid}_URL"; token_var="FIRE_${sid}_TOKEN"
export FIRE_URL=${!url_var:-${FIRE_URL:-}} FIRE_TOKEN=${!token_var:-${FIRE_TOKEN:-}}
if [ "$kind" = перезапуск ] && [ "${WAKE_COMMENT:-1}" != 0 ]; then
  gh pr comment "$pr" -R "$REPO" --body "[guard] перезапуск исполнителя $id ($why): прошлая сессия считается завершённой, новая перехватывает PR."
fi
bash .github/scripts/fire.sh "PR #$pr в $REPO, исполнитель $id, $kind ($why)"
