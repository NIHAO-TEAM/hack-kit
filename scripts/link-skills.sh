#!/usr/bin/env sh
# Создаёт ссылки .claude/skills -> ../.agents/skills в корне и в каждой части репозитория.
# Источник скиллов — .agents/skills; .claude/skills только ссылается на него.
# На Windows используйте scripts/link-skills.ps1 (junction, права администратора не нужны).
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)

for dir in "$root" "$root/frontend" "$root/backend"; do
  [ -d "$dir/.agents/skills" ] || continue
  mkdir -p "$dir/.claude"
  link="$dir/.claude/skills"
  if [ -L "$link" ]; then
    echo "ok: $link"
    continue
  fi
  if [ -e "$link" ]; then
    echo "пропуск: $link существует и это не ссылка — проверьте вручную" >&2
    continue
  fi
  ln -s ../.agents/skills "$link"
  echo "создано: $link -> ../.agents/skills"
done
