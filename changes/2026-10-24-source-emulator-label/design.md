# Дизайн: источник «эмулятор»

## Подход

1. Контракт: `enum: [seed, api, emulator]`; `make generate`. Новое значение enum в ответе `oasdiff` может счесть breaking — тогда метка `contract-breaking` и объяснение ревьюеру: фронт в этом же PR.
2. Сервер: константа `OriginEmulator = "emulator"`; путь эмулятора (`emulator/tick`, `emulator/actions`) передаёт её при записи снимка вместо `OriginAPI`.
3. Фронт: подпись в шапке экрана «Источник» по значению; неизвестное значение — показать как есть, без падения.

## Допущения

- [ПРЕДПОЛОЖЕНО] После `PUT /source/snapshot` извне происхождение снова `api`.

## Риски

- `data-quality-checks` (#82) правит тот же `internal/source/http.go` и `pages/source` — поэтому зависимость.
