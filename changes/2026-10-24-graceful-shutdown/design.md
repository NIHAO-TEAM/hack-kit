# Дизайн: плавная остановка

## Подход

1. `run()` получает `ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`; этот `ctx` уходит в `recalc.Start`, `billing.Run`, `source.OnStart`.
2. `httpx.Readiness` — флаг `atomic.Bool` «готов» и «останавливаюсь»; `/readyz` отвечает `503`, пока нет первого пересчёта или идёт остановка. Признак первого пересчёта — подписка на `analytics.TopicRecalculated` в `main.go` (модули не трогаем).
3. Порядок остановки: сигнал → `ready=false` → `sleep(drain)` → `srv.Shutdown(timeoutCtx)` → ожидание воркеров (`sync.WaitGroup` в `main.go`) → `pool.Close()`.
4. SSE: `srv.RegisterOnShutdown(cancelStreams)`; обработчики потоков слушают общий `streamsCtx` и выходят. Если обработчик SSE лежит в модуле, достаточно, что он слушает `r.Context()`: `Shutdown` сам не закрывает долгие соединения, поэтому `BaseContext` сервера отменяется в `RegisterOnShutdown`.
5. Тест: сервер на `httptest`-листенере, медленный запрос 300 мс и открытый SSE; отправка остановки → медленный запрос получает `200`, SSE закрыт, `/readyz` ответил `503` до закрытия, остановка завершилась быстрее таймаута.

## Допущения

- [ПРЕДПОЛОЖЕНО] Пауза 5 с хватает Traefik/kube-proxy снять под; значения через переменные окружения.
- [ПРЕДПОЛОЖЕНО] Загрузка снимка, попавшая под таймаут, откатывается транзакцией — это уже так (ошибка откатывает всю транзакцию).

## Риски

- `recalc-concurrency` (#146) и `events-stream-emulator` правят `internal/app`; здесь `internal/app` не трогаем, только `main.go`. Конфликт по `main.go` возможен с маршрутом pprof — разрешается слиянием `main`.
