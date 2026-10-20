# Задачи: правило X-Request-Id

## 1. Документация

- [ ] 1.1 Заменить строку про `X-Correlation-Id` в `backend/AGENTS.md` на правило про `X-Request-Id` (формулировка из `proposal.md`) — проверка: `grep -n "Correlation" backend/AGENTS.md` пусто, `grep -n "X-Request-Id" backend/AGENTS.md` находит строку

## 2. Готовность

- [ ] 2.1 `make check` в `backend/` зелёный (код не менялся)
- [ ] 2.2 Архив изменения (`change-archive`): папка в `changes/archive/`; поведение не менялось, файл в `specs/` не добавляется
