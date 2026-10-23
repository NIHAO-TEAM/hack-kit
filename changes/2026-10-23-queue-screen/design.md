# Дизайн: очередь удержания

## Подход

- `entities/customer` — типы из `schema.gen.ts`, `customersQuery(params)` с ключом `['customers', params]`; `entities/summary` — `['summary']`.
- Сортировка и фильтр — параметры URL (TanStack Router search), запрос на сервер; фронт не пересортировывает.
- Тема и компоненты — ADR-0002 и `design-tokens` (`RescueWindow`, `Money`, `Countdown`, `LevelBadge`). Пять состояний экрана — `shared/ui/states`. Моки MSW — свой файл в `src/mocks/` и одна строка регистрации в конце списка обработчиков. Ключи запросов начинаются с корня из `live-updates`.

## Фронт

- Макет: доски C холста. Состояния: загрузка (скелет таблицы), пусто («сегодня некого спасать»), ошибка, нет связи, нет прав.
- Роль: все вошедшие.
