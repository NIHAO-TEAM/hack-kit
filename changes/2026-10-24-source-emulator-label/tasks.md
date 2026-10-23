# Задачи: источник «эмулятор»

Каждая задача проверяема: в конце строки — чем проверить.

## 1. Контракт и данные

- [x] 1.1 `SourceStatus.source` с `emulator` — проверка: `make generate && git diff --exit-code`

## 2. Логика

- [x] 2.1 Снимок эмулятора помечен `emulator` — проверка: тест `internal/source`: после `emulator/tick` статус отдаёт `source: emulator`, после `PUT /source/snapshot` — `api`

## 3. Экран

- [ ] 3.1 Подпись «Эмулятор» в шапке «Источника» — проверка: `pnpm check`, тест страницы

## 4. Готовность

- [ ] 4.1 `make check` и `pnpm check` зелёные
- [ ] 4.2 Сценарий `specs/source` про происхождение снимка
