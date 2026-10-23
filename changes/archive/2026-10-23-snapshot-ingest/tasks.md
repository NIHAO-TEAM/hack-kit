# Задачи: приём снимка

Каждая задача проверяема: в конце строки — чем проверить.

## 1. Контракт и данные

- [x] 1.0 Контракт: `asOf`, `source`, `422` — проверка: `make generate`, `go build ./...`

- [x] 1.1 Запросы `sqlc` модуля `source`: `COPY` трёх таблиц, `snapshots` — проверка: `make generate && git diff --exit-code`
- [x] 1.2 Копия `cases/analizator/data.json` в `internal/source/seed/organizers.json` (`embed`) — проверка: тест «вшитый файл совпадает с `cases/analizator/data.json`»

## 2. Логика

- [x] 2.1 Потоковый разбор с алиасами опечаток — проверка: unit-тест на `data.json` и на тот же файл с полями `pice`/`paymentTem`/`quater_year`/`pefomed`/`autoprolong` → одинаковый результат
- [x] 2.2 Проверки снимка и `errors[]` — проверка: табличный тест (чужой `ownerId`, неизвестный статус, отрицательная сумма, `performedAt` у `not_performed`)
- [x] 2.3 Время модели (`asOf` из тела / максимум дат / пустой снимок) — проверка: unit-тест; на `data.json` без `asOf` → `2026-10-19T14:35:53Z`
- [x] 2.4 Запись в одной транзакции и откат при ошибке — проверка: интеграционный тест «неверный снимок → 422, прежние данные на месте»
- [x] 2.5 `PUT /api/v1/source/snapshot`, `GET /api/v1/source/status` — проверка: тесты через `apitest` по `specs/source.md`
- [x] 2.6 Событие `source.changed` после коммита — проверка: тест с подписчиком шины
- [x] 2.7 Сид при пустой базе (`SEED_ORGANIZERS=true`) — проверка: интеграционный тест «старт на пустой базе → 10/30/100, `asOf` 15:00Z, `source=seed`; второй старт сид не повторяет»

## 3. Экран

- нет

## 4. Готовность

- [x] 4.1 `make check` и `make test-int` зелёные
- [x] 4.2 Сценарии из `specs/` проходят на локальном стенде
- [x] 4.3 README и `.env.example` не трогать (их ведёт параллельное `compose-stand`); пример `curl -X PUT … --data @cases/analizator/data.json` и переменная `SNAPSHOT_MAX_BYTES` — в «Найдено по ходу», пахан перенесёт
