# Дизайн: живое обновление

## Подход

- Бэкенд: подписка на шину `recalculated`, `text/event-stream`, `X-Accel-Buffering: no`, `Cache-Control: no-store`, `Flush` после события, пульс-комментарий раз в 15 с, выход по `ctx.Done()`.
- Фронт: `EventSource('/api/v1/events', {withCredentials: true})`; на событие — `invalidateQueries` по корневым ключам `['customers']`, `['customer']`, `['summary']`, `['model-settings']` — экраны начинают свои ключи с них; состояние связи — контекст `app/realtime`.
- Хук `useChangedRows(items, version)` отдаёт ID изменившихся строк на 2 секунды.

## Решения по ходу

- [ПРЕДПОЛОЖЕНО] Контекст состояния связи и `useChangedRows` лежат в `shared/lib/realtime`, а подписка и плашка — в `app/realtime`: страницы (слой ниже `app`) не могут импортировать из `app`, а подсветка строк нужна им. Расхождение с «контекст `app/realtime`» из подхода; выбрано по правилам FSD.
- Обрыв: `EventSource` переподключается сам; если он закрыт (`readyState = 2`), оболочка поднимает его заново через 3 с. После возврата связи — один перезапрос четырёх корневых ключей.
- Первое сообщение потока — комментарий `: connected`, чтобы клиент сразу увидел открытое соединение, а подписка на шину уже была оформлена.
- Экраны `queue-screen` и `customer-card-screen` ещё не влиты: подсветку строк они подключат сами через `useChangedRows(items, useRealtime().version)`.
