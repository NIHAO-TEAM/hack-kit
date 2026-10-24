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
