#!/usr/bin/env bash
# Разбор правки контракта api/openapi.yaml перед ревью. Модель здесь не участвует: список изменений
# строит oasdiff, а не ревьюер, поэтому «незаметно» ослабить схему нельзя.
#
#   - контракт не менялся — ничего не делает;
#   - иначе комментарий «[ci] контракт: …» со списком изменений (ревьюер сверяет каждую строку с proposal.md);
#   - ломающие изменения (oasdiff breaking, уровень WARN и выше) — метка contract-breaking: review-post.yml
#     ставит по ней manual-merge, такой PR сливает человек. Добавления полей и путей сливаются как обычно.
#
# Сравнение — с общей базой ветки и main (merge-base), а не с текущим main: иначе чужая правка, влитая
# в main после ответвления, выглядела бы как удаление в этом PR.
#
# Окружение: GH_TOKEN, GITHUB_REPOSITORY, RUNNER_TEMP, PR, HEAD_SHA.
set -euo pipefail

REPO=${GITHUB_REPOSITORY:?}
SPEC=api/openapi.yaml
OASDIFF_VERSION=1.33.0
W=$RUNNER_TEMP/contract
mkdir -p "$W"

base=$(gh api "repos/$REPO/compare/main...$HEAD_SHA" --jq .merge_base_commit.sha)
get() { gh api "repos/$REPO/contents/$SPEC?ref=$1" -H "Accept: application/vnd.github.raw" > "$2" 2>/dev/null; }
has_base=0; has_head=0
get "$base" "$W/base.yaml" && has_base=1
get "$HEAD_SHA" "$W/head.yaml" && has_head=1

if { [ "$has_base" = 0 ] && [ "$has_head" = 0 ]; } \
  || { [ "$has_base" = 1 ] && [ "$has_head" = 1 ] && cmp -s "$W/base.yaml" "$W/head.yaml"; }; then
  # Правку контракта могли откатить на новом коммите — метка с прошлого прогона больше не верна.
  gh pr edit "$PR" -R "$REPO" --remove-label contract-breaking >/dev/null 2>&1 || true
  echo "контракт не менялся"; exit 0
fi

gh label create contract-breaking -R "$REPO" --color B60205 >/dev/null 2>&1 || true
breaking=0
{
  echo "[ci] контракт: $SPEC изменён"
  echo "head: $HEAD_SHA"
  echo
  if [ "$has_base" = 0 ]; then
    echo "Контракт создан этим PR. Ломающих изменений нет."
  elif [ "$has_head" = 0 ]; then
    breaking=1
    echo "🚨 Контракт удалён этим PR. Метка \`contract-breaking\`: сливает человек."
  else
    bin=""
    for i in 1 2 3; do
      if curl -fsSL "https://github.com/oasdiff/oasdiff/releases/download/v${OASDIFF_VERSION}/oasdiff_${OASDIFF_VERSION}_linux_amd64.tar.gz" \
           | tar -xz -C "$W" oasdiff; then bin=$W/oasdiff; break; fi
      sleep $((i * 10))
    done
    if [ -z "$bin" ]; then
      # Не смогли проверить — считаем ломающим: лучше лишний раз позвать человека.
      breaking=1
      echo "⚠️ oasdiff не скачался за 3 попытки, проверка не выполнена. Метка \`contract-breaking\`: сливает человек."
    else
      { "$bin" changelog "$W/base.yaml" "$W/head.yaml" 2>&1 || true; } > "$W/changelog.txt"
      # Ослабление проверки запроса oasdiff ломающим не считает (клиенту оно совместимо), но для сервера это
      # снятая валидация: поле стало необязательным или nullable, снят max/pattern, расширен тип или enum.
      relaxed=$(grep -oE '\[request-[a-z-]+\]' "$W/changelog.txt" | tr -d '[]' | sort -u \
        | grep -E '(became-optional|became-nullable|-unset|-removed|generalized|enum-value-added|max(-length|-items|-properties|-contains)?-increased|min(-length|-items|-properties|-contains)?-decreased)$' || true)
      if "$bin" breaking "$W/base.yaml" "$W/head.yaml" --fail-on WARN > "$W/breaking.txt" 2>&1; then
        echo "Ломающих изменений нет (oasdiff breaking, уровень WARN и выше)."
      else
        breaking=1
        echo "🚨 Ломающие изменения. Метка \`contract-breaking\`: после APPROVED PR сливает человек."
        echo
        echo '```'
        head -n 80 "$W/breaking.txt"
        echo '```'
      fi
      if [ -n "$relaxed" ]; then
        breaking=1
        echo
        echo "🚨 Ослаблена проверка запроса: $(printf '%s' "$relaxed" | tr '\n' ' '). Метка \`contract-breaking\`: после APPROVED PR сливает человек."
      fi
      echo
      echo "Все изменения (ревьюер сверяет каждую строку с \`proposal.md\`):"
      echo
      echo '```'
      head -n 150 "$W/changelog.txt"
      echo '```'
    fi
  fi
} > "$W/body.md"

gh pr comment "$PR" -R "$REPO" --body-file "$W/body.md"
if [ "$breaking" = 1 ]; then
  gh pr edit "$PR" -R "$REPO" --add-label contract-breaking
else
  gh pr edit "$PR" -R "$REPO" --remove-label contract-breaking >/dev/null 2>&1 || true
fi
