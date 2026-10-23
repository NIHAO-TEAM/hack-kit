# Дизайн: пересчёт

## Подход

- **`model/store.go`:** `Store.Current(ctx) (risk.Settings, version int64, error)` — последняя строка `model_settings` или `risk.Defaults()` с версией 0; `Store.Save(ctx, s, by string) (version, error)` — `Validate`, новая строка с версией +1 в транзакции, `events.Publish("model.changed", version)`. Экспортируется интерфейс `SettingsReader { Current(ctx) ... }`. HTTP (`GetModelSettings`/`PatchModelSettings`) не трогается — его делает `settings-api`.
- **`source/reader.go`:** `Reader.Load(ctx) (risk.Snapshot, dataVersion int64, error)` — одна транзакция `REPEATABLE READ`: последняя строка `snapshots` и три таблицы. Экспортируется интерфейс `SnapshotReader`. Модуль `analytics` таблиц `source` сам не читает (границы модулей, `backend/AGENTS.md`).
- **`analytics/engine.go`:** `Engine{snap SnapshotReader, set SettingsReader, bus, clk}`; одна горутина: подписка на `source.changed`, `model.changed`; флаг `dirty` + канал-сигнал ёмкостью 1; цикл: снять флаг → `Load` + `Current` → `risk.Score(snapshot, settings, snapshot.AsOf)` → атомарная замена результата (`atomic.Pointer`) с `{Result, DataVersion, SettingsVersion, Version, CalculatedAt, Duration}` → `Publish("recalculated", Version)`. Чтение — `Engine.Latest()` без блокировок. Ошибка загрузки — прежний результат остаётся, ошибка в лог и в счётчик.
- **Подключение:** `analytics.New` создаёт `Engine` из переданных `model` и `source`; в `internal/app/app.go` — передать их и запустить `Engine.Run(ctx)` (одна правка).
- **Время модели** — `snapshot.AsOf` (правка 3 вердикта); `clock` — только для `CalculatedAt` и возраста снимка.

## Отброшенные варианты

| Вариант | Почему нет |
|---|---|
| Пересчёт в обработчике PATCH/PUT | гонка двух одновременных запросов (`r1_B.md` №8) |
| Результаты в таблице | одна реплика (вердикт); пересчёт при старте быстрее записи |
| `LISTEN/NOTIFY` между репликами | отброшено дебатами (ADR-0001, «Отброшено») |

## Допущения

- [ПРЕДПОЛОЖЕНО] `model.changed` и `source.changed` несут версию как `int64` в `payload`.
- [ПРОВЕРЕНО] Шина — `internal/platform/events` (`Publish`, `Subscribe`), медленный подписчик не блокирует (тест `TestSlowSubscriberDoesNotBlock`).

## Решения по ходу

- Задача 1.1 («запросы sqlc»): модуль `source` пишет снимок прямым `pgx` (`service.go`), без sqlc. Выбрал тот же способ для `model/store.go` и `source/reader.go`: SQL — константы рядом с кодом, `make generate` не затрагивается. Причина — единообразие с влитым кодом и меньше риск для параллельных PR (правило 3, пункт 4).
- **Границы модулей (расхождение с «Подходом»).** `backend/.golangci.yml` (depguard) запрещает `model`, `source` и `analytics` импортировать `risk` и друг друга, а файлы пакета, кроме `service.go`, — `pgx`. Гейт не ослабляю (правило 3, пункт 3), поэтому:
  - `Store` живёт в `model/service.go`, `Reader` — в `source/service.go`;
  - `Store` хранит настройки как JSON (`json.RawMessage`) и принимает проверку (`Validator`) от сборки сервера; пустая таблица — `nil` и версия 0;
  - `Reader.Load` отдаёт `source.Snapshot` (`AsOf` — из `snapshots.as_of`, если пусто — `loaded_at`);
  - `analytics.Engine[R]` обобщён по типу результата и получает функцию расчёта и темы шины в конструкторе, на `risk`, `model` и `source` не завязан;
  - склейку (`source.Snapshot` → `risk.Snapshot`, JSON → `risk.Settings` с `risk.Defaults()`, вызов `risk.Score`) делает `internal/app` — ему импорты разрешены.
  Следствие для этапа 4: обработчикам `analytics`, которым нужны поля `risk.Result`, придётся либо получать готовые DTO через `app`, либо менять правила depguard человеком (см. «Найдено по ходу»).

## Риски

- Пропуск события из-за переполнения канала подписчика — флаг `dirty` всё равно выставлен; плюс пересчёт при старте.
