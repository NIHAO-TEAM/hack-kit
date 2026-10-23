# Дизайн: источник «эмулятор»

## Подход

1. Контракт: `enum: [seed, api, emulator]`; `make generate`. Новое значение enum в ответе `oasdiff` может счесть breaking — тогда метка `contract-breaking` и объяснение ревьюеру: фронт в этом же PR.
2. Сервер: константа `OriginEmulator = "emulator"`; путь эмулятора (`emulator/tick`, `emulator/actions`) передаёт её при записи снимка вместо `OriginAPI`.
3. Фронт: подпись в шапке экрана «Источник» по значению; неизвестное значение — показать как есть, без падения.

## Допущения

- [ПРЕДПОЛОЖЕНО] После `PUT /source/snapshot` извне происхождение снова `api`.

## Риски

- `data-quality-checks` (#82) правит тот же `internal/source/http.go` и `pages/source` — поэтому зависимость.

## Решения по ходу

- Подпись для `emulator` уже была в `sourceLabel` («Эмулятор биллинга», из #82); по спеку и макету — «Эмулятор», добавлено пояснение под подписью.
- Происхождение `emulator` получают `emulator/tick` и непустые действия `emulator/actions`; `reset` — `seed`, правки `PATCH /source/*` и `PUT` — `api`.
- [ПРОВЕРЕНО] context7 не нужен: новых API библиотек нет.
