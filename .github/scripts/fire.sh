#!/usr/bin/env bash
# Запуск routine через её API-триггер (https://code.claude.com/docs/en/routines, «Add an API trigger»).
# Использование: FIRE_URL=… FIRE_TOKEN=… .github/scripts/fire.sh "<текст запуска>"
# URL и токен — из секретов репозитория. В лог ничего из них не печатается: логи Actions видны всем, у кого есть доступ к репозиторию.
# Текст запуска routine получает как данные (блок routine-fire-payload), а не как инструкции.
set -euo pipefail

text=${1:?нужен текст запуска}
: "${FIRE_URL:?нет FIRE_URL (секрет FIRE_<ID>_URL)}"
: "${FIRE_TOKEN:?нет FIRE_TOKEN (секрет FIRE_<ID>_TOKEN)}"

body=$(jq -n --arg t "$text" '{text: $t}')
code=$(curl -sS -o "$RUNNER_TEMP/fire-response.json" -w '%{http_code}' -X POST "$FIRE_URL" \
  -H "Authorization: Bearer $FIRE_TOKEN" \
  -H "anthropic-beta: experimental-cc-routine-2026-04-01" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d "$body")

if [ "$code" -lt 200 ] || [ "$code" -ge 300 ]; then
  echo "fire: HTTP $code"
  jq -r '.error.message // empty' "$RUNNER_TEMP/fire-response.json" 2>/dev/null || true
  exit 1
fi
echo "fire: HTTP $code, сессия запущена"
