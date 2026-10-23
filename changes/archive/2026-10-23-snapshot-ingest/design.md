# Дизайн: приём снимка

## Подход

- **Разбор:** `json.Decoder` по токенам — массивы читаются потоково, без загрузки всего тела в память; лимит тела `SNAPSHOT_MAX_BYTES` (по умолчанию 512 МБ).
- **Алиасы:** при разборе ключи `pice→price`, `paymentTem→paymentTerm`, `autoprolong→autoProlong`; значения `quater_year→quarter_year`, `pefomed→performed`, `not_pefomed→not_performed`. Таблица алиасов — одна map в `internal/source/aliases.go`.
- **Проверки** (до записи): уникальность id; `services.ownerId` и `transactions.serviceId` ссылаются на существующие; перечисления; неотрицательные целые; `expiresAt >= createdAt`; `performedAt` пуст ⇔ `not_performed`. Все ошибки собираются в `errors[]` (`{pointer, detail}`), максимум 100.
- **Запись:** одна транзакция: `TRUNCATE transactions, services, accounts` → `COPY` трёх таблиц → строка в `snapshots(version, as_of, loaded_at, source)`. Ошибка — откат, данные прежние.
- **Время модели:** `asOf` из тела; иначе max(`services.createdAt`, `transactions.createdAt`, `transactions.performedAt`); пустой снимок без `asOf` — время приёма. На данных организаторов без `asOf` это 2026-10-19T14:35:53Z, расхождение с сидом (15:00) — меньше 1 € в ожидаемой потере [ПРОВЕРЕНО Скептиком, `r2_B.md`].
- **Сид:** копия `cases/analizator/data.json` вшита через `embed` (путь `internal/source/seed/organizers.json`), `source.OnStart` загружает её, если `snapshots` пуста и `SEED_ORGANIZERS=true`, с `asOf = 2026-10-19T15:00Z` и `source = seed`.
- **Событие:** `events.Publish("source.changed", version)` после коммита.
- **Права:** путь доступен аналитику, `jury`, администратору и сервисному токену — проверку делает `auth.Middleware` (`auth-sessions`); модуль доверяет ей.

## Отброшенные варианты

| Вариант | Почему нет |
|---|---|
| `asOf` = часы сервера по умолчанию | 25.10 расклад уровней на стенде разойдётся со слайдом (P0 раунда 2) |
| Отклонять опечатки кейса | жюри может прислать поля в написании из текста кейса |
| Инкрементальное слияние | кейс требует «полную замену снимка» |

## Допущения

- [ПРОВЕРЕНО] Форма и поля — `cases/analizator/data.json`, `001_schema.up.sql`.
- [ПРЕДПОЛОЖЕНО] Лимит 512 МБ покрывает 100k аккаунтов / 300k услуг / 1M транзакций (344 МБ JSON, замер Скептика).
- [НАДЕЮСЬ] Загрузка 100k ≤ 30 с через `COPY`.

## Риски

- `TRUNCATE` при одновременном чтении — читатели видят либо старый, либо новый снимок целиком (одна транзакция); пересчёт берёт версию из события.

## Решения по ходу

- Контракт расширен в этом PR (вариант B, решение команды): `asOf` в `SnapshotAccepted`, `source` (`seed`/`api`) в `SourceStatus`, ответ `422` у `PUT /source/snapshot`. Поля `errors[]` — `{field, message}`, как в общей схеме `Problem`.
- Опечатки кейса приводятся к именам схемы до разбора тела сгенерированным кодом (`aliases.go`, табличный тест); в контракте только имена схемы, в описании PUT — строка про алиасы.
- [ПРЕДПОЛОЖЕНО] Запись трёх таблиц — `pgx.CopyFrom` напрямую, а не `sqlc :copyfrom`: sqlc не умеет `TRUNCATE`, а запросы к `snapshots` короткие; вопрос ревьюеру в PR.
