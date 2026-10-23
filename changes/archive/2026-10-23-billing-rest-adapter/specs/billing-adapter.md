# billing-adapter: изменения сценариев

## ADDED

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
