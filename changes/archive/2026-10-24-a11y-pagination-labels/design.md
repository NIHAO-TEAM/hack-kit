# Дизайн: подписи пагинации

## Подход

1. `shared/ui/pagination`: обёртка над Mantine `Pagination` с `getControlProps` (`previous`, `next`, `first`, `last` — `aria-label`) и `getItemProps` (`aria-label="Страница N"`, `aria-current` у активной). Экспорт через `shared/ui/index.ts`.
2. `AuditLog` и `QueuePage` берут обёртку вместо Mantine напрямую.
3. `<Code block tabIndex={0} aria-label="Формула модели">`.

## Допущения

- [ПРЕДПОЛОЖЕНО] Подписи — по-русски, как остальной интерфейс.

## Риски

- `QueuePage.tsx` правит `retention-queue-status` (#117): правка здесь — одна строка импорта и тег, конфликт простой.
