# live

### Requirement: обновление без перезагрузки

#### Scenario: изменён коэффициент

- WHEN в одном окне аналитик сохранил новый вес, а в другом открыта очередь маркетолога
- THEN очередь и суммы во втором окне обновились за несколько секунд без перезагрузки, изменившиеся строки подсвечены
- Тест: backend `internal/analytics/events_test.go › TestGetEventsStreamsRecalculated`; фронт `app/realtime/connect.test.ts › на recalculated сбрасывает ключи…`, `shared/lib/realtime/changed-rows.test.ts`

#### Scenario: обрыв связи

- WHEN поток событий оборвался
- THEN видна плашка «нет связи, данные могли устареть»; после восстановления данные перезапрошены, плашка исчезла
- Тест: `app/realtime/connect.test.ts › обрыв даёт offline, возврат связи — один перезапрос`

### Requirement: изменения заметны глазу

Очередь и карточка SHALL подсвечивать то, что изменилось после нового пересчёта.

#### Scenario: пополнение баланса

- WHEN баланс клиента пополнен и пересчёт выдал новую версию
- THEN строка клиента в очереди подсвечена около 2 секунд, остальные — нет; в карточке подсвечены изменившиеся числа шапки и действие
- Тест: `pages/queue/ui/QueueTable.test.tsx`, `app/customer-screen.test.tsx › новая версия пересчёта подсвечивает…`

### Requirement: рассылка событий не блокирует издателя и считает потери

Шина событий SHALL рассылать событие без общей блокировки на всех подписчиков и считать каждое выброшенное из переполненного буфера событие в `churn_events_dropped_total`.

#### Scenario: медленный подписчик

- WHEN подписчик не читает, а издатель публикует 100 событий в тему
- THEN издатель не ждёт, `churn_events_dropped_total{topic}` равен числу выброшенных, подписчик получает последние события
- Тест: `internal/platform/events › TestSlowSubscriberCountsDrops`, `TestSlowSubscriberDoesNotBlock`

#### Scenario: подписка и отписка под потоком

- WHEN 8 горутин публикуют, а 1 000 подписчиков подписываются и отписываются
- THEN нет паники и гонок (`-race`)
- Тест: `internal/platform/events › TestPublishWhileSubscribeCancel`
