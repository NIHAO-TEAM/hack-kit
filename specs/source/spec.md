# source

### Requirement: полная замена снимка

Система SHALL заменять аккаунты, услуги и транзакции целиком по `PUT /api/v1/source/snapshot`.

#### Scenario: загрузка набора организаторов

- WHEN аналитик отправляет `PUT /api/v1/source/snapshot` с телом `cases/analizator/data.json`
- THEN ответ `200`, в теле номер новой версии, `asOf` и счётчики 10 / 30 / 100; `GET /api/v1/source/status` показывает эту версию и `source = api`
- Тест: `TestPutSnapshotOrganizers`

#### Scenario: замена, а не слияние

- WHEN после загрузки набора организаторов загружен снимок с одним аккаунтом
- THEN в системе один аккаунт, прежние 10 не видны
- Тест: `TestPutSnapshotReplaces`

#### Scenario: неверный снимок

- WHEN в снимке услуга ссылается на несуществующий `ownerId`
- THEN ответ `422` `problem+json`, в `errors[]` указан путь к полю; данные и версия прежние
- Тест: `TestPutSnapshotInvalidKeepsOld`

#### Scenario: опечатки из текста кейса

- WHEN снимок использует поля `pice`, `paymentTem`, `autoprolong` и значения `quater_year`, `pefomed`
- THEN снимок принимается так же, как с именами схемы, а ответы API используют имена схемы
- Тест: `TestPutSnapshotTypos (разбор: TestParseTypos)`

### Requirement: время модели из снимка

Система SHALL считать модель на времени снимка, а не на часах сервера.

#### Scenario: `asOf` передан

- WHEN снимок содержит `asOf = 2026-10-19T15:00:00Z`
- THEN `GET /api/v1/source/status` возвращает `asOf = 2026-10-19T15:00:00Z`
- Тест: `TestPutSnapshotAsOfFromBody`

#### Scenario: `asOf` не передан

- WHEN загружен `cases/analizator/data.json` без поля `asOf`
- THEN `asOf` равен самой поздней дате события снимка: `2026-10-19T14:35:53Z`
- Тест: `TestPutSnapshotOrganizers, TestModelTime`

### Requirement: сид на пустой базе

#### Scenario: первый запуск

- WHEN система стартует на пустой базе с `SEED_ORGANIZERS=true`
- THEN загружен набор организаторов, `asOf = 2026-10-19T15:00:00Z`, `source = seed`
- Тест: `TestSeed`

#### Scenario: повторный запуск

- WHEN система перезапускается с уже загруженным снимком
- THEN снимок и его версия не меняются
- Тест: `TestSeed`

### Requirement: происхождение снимка «эмулятор»

Система SHALL помечать снимок, записанный эмулятором, происхождением `emulator`.

#### Scenario: жюри включает эмулятор

- WHEN выполнен `POST /api/v1/source/emulator/tick`
- THEN `GET /api/v1/source/status` отдаёт `source: emulator`, а после `PUT /api/v1/source/snapshot` — снова `api`
- AND экран «Источник» показывает в шапке «Эмулятор» с пояснением
- Тест: `TestEmulatorOrigin`, `source-screen.test.tsx` («источник «%s»…»)
