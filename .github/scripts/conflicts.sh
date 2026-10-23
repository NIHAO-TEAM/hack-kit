#!/usr/bin/env bash
# Конфликты с main: будит исполнителя, если его PR после очередного слияния в main стал CONFLICTING.
#
# Зачем: GitHub не шлёт pull_request для PR в конфликте, CI не запускается, ревью не приходит, исполнитель
# ждёт итога вечно, а сторож считает, что ход ревьюера (23.10: #55, #58, #64 стояли больше 45 минут).
#
# Смотрит открытые PR с go:<id>, без needs-human, не черновики. CONFLICTING и на этот head main ещё не будили —
# комментарий «[guard] перезапуск исполнителя <id> (конфликт с main @<sha>)» и /fire. Исполнитель по шагу 0
# executor.md перехватывает PR и вливает main («Конфликты с main»). Счётчик перезапусков сторожа это не трогает.
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
  marker="[guard] перезапуск исполнителя $id (конфликт с main @$short)"
  done_before=$(gh api "repos/$REPO/issues/$pr/comments" --paginate \
    --jq "[.[] | select(.user.login == \"github-actions[bot]\" and (.body | startswith(\"$marker\")))] | length" | tail -n1)
  [ "${done_before:-0}" -eq 0 ] || continue

  sid=$(printf '%s' "$id" | tr 'a-z-' 'A-Z_')
  url_var="FIRE_${sid}_URL"; token_var="FIRE_${sid}_TOKEN"
  export FIRE_URL=${!url_var:-} FIRE_TOKEN=${!token_var:-}
  gh pr comment "$pr" -R "$REPO" --body "$marker: PR в конфликте с main, CI и ревью не запустятся, пока его не разрешить. Влей main по разделу «Конфликты с main» executor.md, прогони тесты, push."
  if bash .github/scripts/fire.sh "PR #$pr в $REPO, исполнитель $id, перезапуск (конфликт с main @$short): влей main и разреши конфликт"; then
    woken=$((woken + 1)); echo "#$pr -> $id: конфликт с main @$short"
  else
    gh label create needs-human --color B60205 -R "$REPO" >/dev/null 2>&1 || true
    gh pr edit "$pr" -R "$REPO" --add-label needs-human || true
    gh pr comment "$pr" -R "$REPO" --body "[guard] не смог разбудить $id через API-триггер (секреты FIRE_${sid}_*). Нужен человек."
  fi
done
echo "разбужено: $woken"
