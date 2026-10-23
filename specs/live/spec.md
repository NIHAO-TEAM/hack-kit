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
