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

### Requirement: данные из REST

Система SHALL при заданном `BILLING_URL` раз в `BILLING_POLL_SECONDS` забирать снимок той же формы, что `data.json`, с `Authorization: Bearer BILLING_TOKEN`, и писать его с `source = api`, если тело изменилось.

#### Scenario: биллинг изменил данные

- WHEN биллинг отдаёт снимок с новым балансом клиента 9
- THEN через интервал опроса очередь пересчитана без действий пользователя
- Тест: `TestPollWritesNewSnapshotAndSkipsSame`

#### Scenario: тело не менялось

- WHEN биллинг отдаёт то же тело
- THEN новый снимок не пишется, версия прежняя
- Тест: `TestPollWritesNewSnapshotAndSkipsSame`

### Requirement: сбой источника не стирает данные

#### Scenario: биллинг недоступен или отдал плохой снимок

- WHEN биллинг отвечает 502, битым JSON или снимком, не прошедшим проверку
- THEN данные, версия и `asOf` прежние, `churn_billing_poll_errors_total` растёт, `polling.lastError` заполнен; экран пишет «данные от HH:MM»
- Тест: `TestPollFailureKeepsDataAndCountsErrors`, `TestMetricsCountPollErrors`

### Requirement: ручная загрузка главнее опроса

#### Scenario: пауза после ручного PUT

- WHEN аналитик загрузил снимок через `PUT /api/v1/source/snapshot`
- THEN `polling.paused = true`, опрос биллинг не трогает; после `PUT /api/v1/source/polling {paused:false}` ближайший опрос возвращает данные биллинга
- Тест: `TestManualPutPausesPollingUntilResumed`

#### Scenario: опрос не настроен

- WHEN `BILLING_URL` пуст
- THEN в статусе нет `polling`, `PUT /api/v1/source/polling` отвечает 404, маркетолог получает 403
- Тест: `TestPollingEndpointWithoutAdapterIs404`

### Requirement: чужой снимок не ломает стенд

Система SHALL на любой снимок отвечать либо приёмом с согласованными числами, либо ошибкой (400 — тело не JSON и не форма снимка, 422 — ошибки полей), оставляя прежние данные.

#### Scenario: случайный вход

- WHEN в `PUT /source/snapshot` приходит произвольный байтовый поток
- THEN разбор и проверка не падают; ответ 200, 400 или 422; принятый снимок проходит модель без паники, с согласованными числами; при 400 и 422 данные прежние
- Тест: `FuzzParseSnapshot`, `FuzzScoreAcceptedSnapshot`, `TestPutSnapshotInvalidKeepsOld`

#### Scenario: сумма выше предела

- WHEN в снимке сумма (баланс, вывод, пополнения, бонус или цена услуги) больше 1 000 000 000 центов
- THEN ответ 422 с ошибкой поля; иначе платежи на горизонт переполнили бы int64 и дали отрицательные потери
- Тест: `TestValidateMoneyLimit`

#### Scenario: крайний набор

- WHEN загружен пустой снимок, снимок только из удалённых услуг, без платежей, с нулевыми ценами, датами в будущем или только почасовыми услугами
- THEN сумма потерь очереди равна сводке и `/metrics`, скор каждого клиента в [0, 100] процентов или `null`, отрицательных потерь нет
- Тест: `TestEdgeSnapshotsConsistent`

#### Scenario: большой снимок

- WHEN загружен снимок на 10 000 аккаунтов
- THEN стенд принимает его и пересчитывает за порядка секунды; время и память — в `docs/load/2026-10-23-large-snapshot.md`
- Тест: `TestLargeSnapshotLoad` (теги `integration load`), `TestScoreScalesWithAccounts`

### Requirement: этапы загрузки снимка измеряются

Сервер SHALL записывать длительность каждого этапа загрузки снимка в лог и в метрику.

#### Scenario: загрузка снимка

- WHEN принят снимок
- THEN `/metrics` содержит `churn_snapshot_ingest_stage_seconds` по этапам `parse`, `validate`, `lock`, `truncate`, `copy_*`, `sequences`, `commit`
- Тест: `TestStagesObserveAndLint`

### Requirement: загрузка во время пересчёта

Загрузка нового снимка SHALL завершаться, не дожидаясь окончания пересчёта по предыдущему снимку, и не оставлять результат старой версии.

#### Scenario: две загрузки подряд

- WHEN второй снимок загружается, пока идёт пересчёт по первому
- THEN вторая загрузка принята, итоговая версия модели соответствует второму снимку
- Тест: `TestLargeSnapshotLoad` с `LOAD_REPEAT=1` (теги `integration load`, замер, не гейт)
