# Дизайн: живое обновление

## Подход

- Бэкенд: подписка на шину `recalculated`, `text/event-stream`, `X-Accel-Buffering: no`, `Cache-Control: no-store`, `Flush` после события, пульс-комментарий раз в 15 с, выход по `ctx.Done()`.
- Фронт: `EventSource('/api/v1/events', {withCredentials: true})`; на событие — `invalidateQueries` по корневым ключам `['customers']`, `['customer']`, `['summary']`, `['model-settings']` — экраны начинают свои ключи с них; состояние связи — контекст `app/realtime`.
- Хук `useChangedRows(items, version)` отдаёт ID изменившихся строк на 2 секунды.
