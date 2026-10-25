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
- THEN `/metrics` содержит `churn_snapshot_ingest_stage_seconds` по этапам `parse`, `validate`, `lock`, `truncate`, `copy_accounts`, `copy_services`, `copy_transactions`, `sequences`, `commit` и `churn_snapshot_ingest_peak_heap_bytes` (пик кучи за последнюю загрузку); этапы идут одновременно: `parse` и `validate` — время работы без ожиданий, `copy_*` — вместе с ожиданием разбора
- Тест: `TestSnapshotIngestStagesAndSecondLoad`, `TestSnapshotIngestMetrics` (метрики на стенде), `TestStagesObserveAndLint` (формат метрики)

### Requirement: загрузка во время пересчёта

Загрузка нового снимка SHALL завершаться, не дожидаясь окончания пересчёта по предыдущему снимку, и не оставлять результат старой версии.

#### Scenario: две загрузки подряд

- WHEN второй снимок загружается, пока идёт пересчёт по первому
- THEN вторая загрузка принята, итоговая версия модели соответствует второму снимку
- Тест: `TestSnapshotIngestStagesAndSecondLoad` (две загрузки подряд, версия сводки равна версии второго ответа); замер времени — `TestLargeSnapshotLoad` с `LOAD_REPEAT=1`

### Requirement: снимок загружается потоком

Приём снимка SHALL разбирать тело потоком и передавать записи в `COPY` через каналы с ограниченными буферами, не удерживая весь снимок в памяти; результат, проверки и коды ошибок не меняются.

#### Scenario: большой снимок

- WHEN загружен снимок на 100 000 аккаунтов (генератор организаторов)
- THEN загрузка успешна, данные в таблицах совпадают со снимком, пик памяти на загрузку ниже записанной в тесте границы (32 МБ на 100k, 64 МБ на 300k аккаунтов)
- Тест: `TestSnapshotIngestPeakMemory` (300k — при `LOAD_300K=1`)

#### Scenario: порядок ключей

- WHEN в теле массивы идут в порядке `transactions`, `services`, `accounts`
- THEN загрузка успешна с тем же результатом
- Тест: `TestSnapshotIngestAnyKeyOrder`, `TestParseStreamAnyKeyOrder`, `TestValidatorDeferredRefs`

#### Scenario: битая запись

- WHEN запись с ошибкой стоит в середине потока или тело оборвано
- THEN ответ с прежним кодом ошибки (422 или 400), таблицы и версия остались прежними
- Тест: `TestSnapshotIngestRollsBackMidStream`

#### Scenario: зависший клиент

- WHEN клиент начал отдавать тело снимка и замолчал дольше срока на тело (`SNAPSHOT_BODY_TIMEOUT_SEC`, по умолчанию 120 с)
- THEN ответ 400, транзакция откатана, таблицы и версия прежние
- Тест: `TestSnapshotIngestBodyTimeout`

### Requirement: пачка событий извне

Сервер SHALL принимать пачку событий биллинга `POST /source/events` (NDJSON или JSON-массив, до 5 000 событий и 2 МБ) тем же пакетным путём записи, что события эмулятора, не применяя повторно событие с уже принятым `eventId`. Доступ: служебный токен источника, аналитик, `jury`, администратор; маркетологу 403, без сессии 401.

#### Scenario: повтор пачки

- WHEN биллинг отправляет ту же пачку из 500 событий дважды
- THEN первый ответ `accepted = 500`, второй `duplicate = 500`, данные изменились один раз
- Тест: `TestPostSourceEventsRepeatedBatch`, `TestPostSourceEventsAppliesOnceAndDeduplicates` (интеграционные), `TestProcessDuplicates`

#### Scenario: строка с ошибкой

- WHEN в пачке есть строка с битым JSON, неизвестным видом события или недопустимым значением
- THEN ответ 202, годные события приняты, ошибочные — в `rejected` с номером строки
- Тест: `TestProcessPartialRejected`, `TestParseEventRejects`

#### Scenario: пределы запроса

- WHEN в запросе больше 5 000 событий или тело больше 2 МБ
- THEN ответ 413, ничего не принято
- Тест: `TestReadEventsCountLimit`, `TestPostSourceEventsBodyOverLimit`

#### Scenario: перегрузка

- WHEN очередь приёма полна дольше 1 с
- THEN ответ `503` с `Retry-After`, ни одно событие запроса не применено, `eventId` запроса свободны для повтора
- Тест: `TestProcessBackpressureAllOrNothing`, `TestPostSourceEvents503RetryAfter`

#### Scenario: права

- WHEN пачку шлёт токен источника, аналитик или `jury`
- THEN ответ 202; маркетологу 403, без сессии и с токеном метрик 401
- Тест: `TestPermissionMatrixCoversEveryOperation`, `TestSourceTokenOpensOnlySnapshot`, `TestPostSourceEventsRoles` (интеграционный)

### Requirement: состояние прогона потока одинаково на всех репликах

Сервер SHALL отдавать на `GET /source/emulator/stream` одно и то же состояние прогона с любой реплики, а прогон, реплика которого остановилась, — как `failed`.

#### Scenario: опрос не той реплики

- WHEN прогон идёт на реплике A, а `GET /source/emulator/stream` попал на реплику B
- THEN ответ — `running` с тем же `runId` и счётчиками не старше 1 с; по окончании — `done` с отчётом
- Тест: `TestStreamStateVisibleFromAnotherReplica` (интеграционный); `Deploy chaos`

#### Scenario: ведущая реплика умерла

- WHEN реплика, которая вела прогон, остановилась и не обновляла состояние больше 10 с
- THEN ответ — `failed` с ошибкой об остановленной реплике, новый прогон можно запустить
- Тест: `TestStreamStateOfDeadReplicaIsFailed`, `TestStreamStateWithLiveLeaderStaysRunning` (интеграционные)

### Requirement: реплика узнаёт о снимке, загруженном через другую

Сервер SHALL пересчитать данные на каждой реплике после загрузки снимка через любую из них, без перезапуска.

#### Scenario: снимок загружен через другую реплику

- WHEN снимок загружен через реплику A, а реплика B отвечает на чтение
- THEN в течение нескольких секунд (`RECALC_PEER_POLL`, 2 с) B пересчитывает данные, и чтение на ней отвечает 200 вместо 503 «расчёт ещё не готов»
- Тест: `TestReplicaSeesSnapshotLoadedViaPeer`; `Deploy chaos`
