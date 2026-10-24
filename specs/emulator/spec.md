# emulator

### Requirement: изменить данные на стенде

Система SHALL давать аналитику и `jury` менять данные без своего биллинга; каждое действие пишет снимок тем же путём, что `PUT /source/snapshot`, публикует `source.changed`, ответ помечен заголовком `X-Billing-Emulator: simulated` («имитация биллинга»).

#### Scenario: включить автопродление

- WHEN аналитик применяет «включить автопродление» к клиенту 1
- THEN через несколько секунд потеря клиента 1 снизилась на 375,08 €, очередь обновилась без перезагрузки
- Тест: `TestEmulatorActionsAndReset` (`backend/internal/source/emulator_http_test.go`) — флаг у услуг клиента; пересчёт держит `recalc-engine`

#### Scenario: сброс

- WHEN аналитик нажимает «сбросить»
- THEN данные вернулись к набору организаторов и стенда
- Тест: `TestEmulatorActionsAndReset`

#### Scenario: ход времени

- WHEN аналитик сдвигает время на N часов
- THEN почасовые услуги с автопродлением списали N цен, услуги без автопродления или без денег ушли в `prolong_wait`, `prolong_wait` старше 5 дней — в `deleted`, `asOf` вырос на N часов
- Тест: `TestAdvance`, `TestAdvanceDeletesAfterFiveDays` (`backend/internal/source/emulator_test.go`), `TestEmulatorTickMovesTime`

#### Scenario: маркетолог

- WHEN маркетолог вызывает tick или actions
- THEN ответ `403`; без сессии — `401`
- Тест: `TestEmulatorPermissions`

### Requirement: сумма пополнения доходит до копейки

Эмулятор SHALL пополнять баланс на ровно ту сумму, которая введена в поле «Сумма, €».

#### Scenario: сумма с копейками

- WHEN на экране «Источник» введена сумма 529,84 и нажато «Пополнить»
- THEN в запрос уходит 52984 цента, наблюдение в карточке клиента показывает «Баланс пополнен на 529,84 €»
- Тест: `пополнение на «%s» € уходит в запрос как %d центов` (`frontend/src/app/source-screen.test.tsx`); наблюдение в карточке — `Demo e2e`

### Requirement: поток событий пользователей

Система SHALL давать аналитику и `jury` прогнать поток из 100–5 000 событий (по умолчанию 2 500) над текущим снимком, применяя их тем же путём, что правки услуг и аккаунтов (`patchService`, `patchAccount`, одна запись снимка на событие), и вернуть отчёт: длительность, события в секунду, p50/p95/максимум применения, число пересчётов, задержку до обновлённой очереди, смену уровней и потери до и после.

#### Scenario: прогон 2 500 событий

- WHEN аналитик запускает поток из 2 500 событий с зерном 1
- THEN все события применены без ошибок, отчёт заполнен, ход идёт в SSE `streamProgress`, очередь у маркетолога обновляется по `recalculated`
- Тест: `TestStreamReportOn2500Events` (`backend/internal/source/stream_report_integration_test.go`), `TestStreamRunsAndReportsConsistency` (`stream_http_test.go`)

#### Scenario: итог совпадает с пересчётом с нуля

- WHEN поток закончен
- THEN отчёт показывает проверку согласованности с разницей 0 центов между результатом сервиса и свежим расчётом над итоговым снимком
- Тест: `TestStreamRunsAndReportsConsistency`, `TestStreamReportOn2500Events`

#### Scenario: то же зерно

- WHEN поток строится дважды с тем же зерном на одном снимке
- THEN последовательность событий одна и та же, каждое событие применимо после предыдущих
- Тест: `TestStreamIsDeterministic`, `TestStreamEventsAreApplicableInOrder` (`backend/internal/source/stream_gen_test.go`)

#### Scenario: права, границы и одновременность

- WHEN маркетолог запускает поток
- THEN ответ `403`; без сессии `401`; число событий вне 100–5 000 или скорость вне 0–2 000 — `400`; при идущем прогоне повторный запуск даёт `409`
- Тест: `TestStreamPermissionsAndLimits`, `TestStreamAllowsOnlyOneRun`

#### Scenario: метрики потока

- WHEN поток применил события
- THEN `/metrics` содержит `churn_emulator_stream_events_total{result}` и гистограмму `churn_emulator_stream_apply_seconds`
- Тест: `TestStreamMetrics`

### Requirement: панель потока на экране «Источник»

Экран «Источник» SHALL показывать аналитику и `jury` панель с запуском потока, ходом выполнения и итоговой таблицей; маркетолог экрана не видит.

#### Scenario: запуск из интерфейса

- WHEN аналитик нажимает «Прогнать 2 500 событий»
- THEN в запрос уходят число, скорость и зерно; идущий прогон показывает полосу хода и «Применено N из M»; по окончании — таблица отчёта и отметка «Согласованность: пересчёт с нуля совпал», а при расхождении — ошибка с суммой
- Тест: `панель «Поток событий»` (`frontend/src/app/stream-panel.test.tsx`), `на streamProgress кладёт кадр хода потока в кэш` (`frontend/src/app/realtime/connect.test.ts`)

#### Scenario: маркетолог

- WHEN маркетолог открывает `/source`
- THEN вместо экрана — «Недостаточно прав», панели нет
- Тест: `маркетолог экрана «Источник» и панели не видит` (`stream-panel.test.tsx`)

