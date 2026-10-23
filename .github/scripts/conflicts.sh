#!/usr/bin/env bash
# Конфликты с main: будит исполнителя, если его PR после очередного слияния в main стал CONFLICTING.
#
# Зачем: GitHub не шлёт pull_request для PR в конфликте, CI не запускается, ревью не приходит, исполнитель
# ждёт итога вечно, а сторож считает, что ход ревьюера (23.10: #55, #58, #64 стояли больше 45 минут).
#
# Смотрит открытые PR с go:<id>, без needs-human, не черновики. CONFLICTING и на этот head main ещё не писали —
# wake-exec.sh (аренда PR):
#   - сессия жива — напоминание «[guard] <id>: снова конфликт с main @<sha>» без новой сессии: подписанная
#     сессия видит комментарий и вливает main (executor.md, «Ожидание»);
#   - запуск уже в пути — ничего: новая сессия вольёт свежий main сама;
#   - иначе «[guard] перезапуск исполнителя <id> (конфликт с main @<sha>)» и /fire; исполнитель по шагу 0
#     перехватывает PR и вливает main («Конфликты с main»). Счётчик перезапусков сторожа это не трогает.
# 24.10: без аренды каждое слияние в main (раз в 1–2 минуты) поднимало ещё одну сессию — 3–4 на PR.
#
# Окружение: GH_TOKEN, GITHUB_REPOSITORY, MAIN_SHA, FIRE_<ID>_URL / FIRE_<ID>_TOKEN (по именам из conflicts.yml).
set -euo pipefail

REPO=${GITHUB_REPOSITORY:?}
MAIN_SHA=${MAIN_SHA:?}
short=${MAIN_SHA:0:7}

mapfile -t PRS < <(gh pr list -R "$REPO" --state open --limit 300 --json number,isDraft,labels \
  --jq '.[] | select((.isDraft | not) and ([.labels[].name] | index("needs-human") | not))
        | ([.labels[].name | select(startswith("go:"))] | first) as $go | select($go != null)
        | "\(.number)|\($go | ltrimstr("go:"))"')

# GitHub считает слияемость лениво: первый запрос часто даёт UNKNOWN.
mergeable_of() {
  local pr=$1 m i
  for i in 1 2 3 4 5 6; do
    m=$(gh pr view "$pr" -R "$REPO" --json mergeable --jq .mergeable)
    [ "$m" != "UNKNOWN" ] && { printf '%s' "$m"; return; }
    sleep 10
  done
  printf 'UNKNOWN'
}

woken=0
for p in "${PRS[@]}"; do
  IFS='|' read -r pr id <<<"$p"
  [ "$(mergeable_of "$pr")" = "CONFLICTING" ] || continue
  done_before=$(gh api "repos/$REPO/issues/$pr/comments" --paginate     --jq "[.[] | select(.user.login == \"github-actions[bot]\" and (.body | startswith(\"[guard]\")) and (.body | contains(\"конфликт с main @$short\")))] | length" | tail -n1)
  [ "${done_before:-0}" -eq 0 ] || continue

  rc=0
  bash .github/scripts/wake-exec.sh "$pr" "$id" перезапуск "конфликт с main @$short" || rc=$?
  case $rc in
    0) woken=$((woken + 1)); echo "#$pr -> $id: конфликт с main @$short, новая сессия" ;;
    10) gh pr comment "$pr" -R "$REPO" --body "[guard] $id: снова конфликт с main @$short. Сессия жива, новую не запускаю: влей main по разделу «Конфликты с main» executor.md, прогони тесты, push."
        echo "#$pr -> $id: сессия жива, напоминание" ;;
    11) echo "#$pr -> $id: запуск уже в пути" ;;
    *) gh label create needs-human --color B60205 -R "$REPO" >/dev/null 2>&1 || true
       gh pr edit "$pr" -R "$REPO" --add-label needs-human || true
       gh pr comment "$pr" -R "$REPO" --body "[guard] не смог разбудить $id через API-триггер (секреты FIRE_$(printf '%s' "$id" | tr 'a-z-' 'A-Z_')_*). Нужен человек." ;;
  esac
done
echo "разбужено: $woken"
