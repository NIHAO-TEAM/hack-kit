# retention-tasks

### Requirement: взять клиента в работу

Сервис SHALL хранить задачу удержания по клиенту: ответственный, статус, срок, заметка, история.

#### Scenario: первая задача

Тест: `TestTaskTakeAndBaseline` (internal/app/tasks_integration_test.go)

- WHEN маркетолог берёт в работу клиента 9
- THEN создана задача «В работе», ответственный — он, срок — завтра 18:00 МСК, в строке клиента 9 поле `task`

#### Scenario: повтор и гонка

Тест: `TestTaskTakeRace, TestTaskTakeAndBaseline` (internal/app/tasks_integration_test.go)

- WHEN два запроса «взять в работу» по одному клиенту пришли одновременно
- THEN задача одна, второй ответ 409 с этой задачей

#### Scenario: устаревшая версия

Тест: `TestTaskPatch` (internal/app/tasks_integration_test.go)

- WHEN сотрудник меняет задачу по версии, которую уже изменил другой
- THEN 409 с логином другого и текущей задачей, данные не изменились

#### Scenario: права

Тест: `TestTaskPermissions` (internal/app/tasks_integration_test.go)

- WHEN аналитик пытается взять клиента в работу
- THEN 403, задачи нет

#### Scenario: исходное состояние

- WHEN клиента берут в работу
- THEN в задаче записаны время и версия пересчёта, скор, уровень, ожидаемая потеря, главное действие, баланс и по услугам статус, автопродление, срок; они совпадают с карточкой на этот момент и дальше не меняются

Тест: `TestTaskTakeAndBaseline`

#### Scenario: проверки правки

- WHEN задачу закрывают без итога или откладывают без нового срока
- THEN 400, данные не изменились

Тест: `TestTaskPatch`, `TestApplyPatch`

#### Scenario: закрытая задача

- WHEN в закрытую задачу пишут правку или контакт
- THEN 409 с её текущим состоянием; клиента можно взять в работу заново — это новая задача

Тест: `TestTaskPatch`

### Requirement: история контактов

#### Scenario: контакт и заметка

- WHEN сотрудник записывает контакт (канал `mail`, `phone`, `messenger`, `other`) или заметку
- THEN событие в истории задачи по порядку, контакт без канала — 400

Тест: `TestTaskEventsAndList`, `TestValidateEvent`

### Requirement: список и фильтры

#### Scenario: задачи в очереди клиентов

- WHEN запрашивают `GET /customers?task=none|open|mine`
- THEN строки содержат `task` (id, статус, ответственный, срок, просрочена), фильтр применяется до страниц, `total` считается после фильтра

Тест: `TestTaskEventsAndList`

#### Scenario: фильтры списка задач

- WHEN запрашивают `GET /tasks` с `status`, `assignee=me`, `overdue=true`
- THEN приходят только подходящие задачи, свежие правки сверху

Тест: `TestTaskEventsAndList`

### Requirement: журнал и живое обновление

#### Scenario: журнал и SSE

- WHEN задачу создают, меняют или пишут в историю
- THEN в журнале запись (`taskCreated`, `taskChanged`, `taskRecorded`) без email клиента и текстов, в потоке `/events` событие `tasks` с `type`, `taskId`, `customerId`

Тест: `TestTaskAuditAndSSE`

### Requirement: задачи переживают замену снимка

#### Scenario: клиент пропал из снимка

Тест: `TestTaskSurvivesSnapshotReplace` (internal/app/tasks_integration_test.go)

- WHEN загружен снимок без клиента, по которому открыта задача, или у того же ID другой email
- THEN задача сохранилась с отметкой `customerMissing` и не привязана к другому клиенту
