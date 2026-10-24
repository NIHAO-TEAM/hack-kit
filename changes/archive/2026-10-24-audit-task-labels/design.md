# Дизайн: подписи журнала по действию

## Подход

1. `fieldLabel(field, action)` и `changeText(change, action)`: для `task*` — словарь полей задачи, значения через подписи из `entities/retention-task`; для остальных — как сейчас.
2. `dueAt` — `formatDateTime` из `shared/lib/format`.
3. Импорт сущности в виджет — по слоям FSD (`widgets` → `entities` разрешено, `steiger` зелёный).

## Допущения

- нет

## Риски

- Если в `main` формат полей задачи в журнале другой (имена полей), — брать фактические из `retention/task.go`.
