# Конкуренты по кейсу «риск оттока aéza» (снимок 24.10, 19:30 МСК)

Только чтение публичных репозиториев, ветка по умолчанию, `git clone --depth 1`. Числа конкурентов взяты из их README и сценариев нагрузки, мы их не перепроверяли. Это гипотезы о чужих системах, а не замеры.

## Кто что сделал

| Репозиторий | Стек ядра | Данные и поток | Нагрузка (с их слов) | Чем сильны |
|---|---|---|---|---|
| [dimon0804/ChurnRadar](https://github.com/dimon0804/ChurnRadar) | микросервисы: Python FastAPI (`api`, `scoring`, `ingest`, `alerts`, `simulator`, `auth`), Go — только `notifier` (SSE); NATS JetStream, Redis, PostgreSQL, Traefik ForwardAuth, Keycloak | события через JetStream, DLQ, идемпотентность `processed_events`; генератор до 100 000 клиентов | k6: рейтинг до 200 RPS, приём 20 запросов/с × пачка 5 (≈ 100 событий/с), 500 SSE-клиентов; в `tests/load/README.md` строка «Полный пересчёт 100k: X с» не заполнена | Helm, HPA 2–6 реплик, PDB, `make k8s-demo` с удалением пода под нагрузкой; 454 файла, из них ≈ 120 тестовых по именам; главный соперник по архитектуре |
| [Unkno394/aeza-intelligence](https://github.com/Unkno394/aeza-intelligence) | Rust (axum, tokio), PostgreSQL | онлайн-модель поведения клиента, симулятор рынка, сбор тарифов конкурентов | ограничения 128 КиБ тела, 2 воркера (`429`) | Rust; уходит от кейса в «рыночную аналитику» |
| [Pixelminds-hub/aeza-analizer](https://github.com/Pixelminds-hub/aeza-analizer) | Python FastAPI, SQLite/PostgreSQL | набор организаторов (10/30/100), расширенный — 60 клиентов | чтение ≈ 115–120 RPS на 4 vCPU | готовы слайды, тексты для спикера и демо-сценарий |
| [WhyFovius/kontur](https://github.com/WhyFovius/kontur) | Python FastAPI, SQLite | сид 8 клиентов; часть экранов работает по топ-100 | — | панель и карточка, проверки снимка |
| [blacerxd/Churn-Risk-Analyzer](https://github.com/blacerxd/Churn-Risk-Analyzer) | Django, Celery, RabbitMQ, PostgreSQL | — | сценарии k6 есть | очередь задач Celery |
| [D1m4ik-creator/Hackaton_Autumm_2026](https://github.com/D1m4ik-creator/Hackaton_Autumm_2026) | FastAPI, PostgreSQL, React | — | — | — |
| [PANDORA615/hakaton](https://github.com/PANDORA615/hakaton) | FastAPI, PostgreSQL/SQLAlchemy, React, OIDC | набор организаторов и демо-клиенты | — | — |
| [lekseyz/HackBack_2026](https://github.com/lekseyz/HackBack_2026) | C# ASP.NET | — | — | — |
| [SergeyKitov/churn](https://github.com/SergeyKitov/churn) | только README: LightGBM, FastAPI в планах | срезы по `cutoff` | — | другой подход: вероятность ухода |
| [Def0ltT3pe/SaveFlow](https://github.com/Def0ltT3pe/SaveFlow) | Python (Alembic), фронт на TS | — | — | скоринговая карта (найден поиском) |

Поиск по словам «aeza», «отток», «анализатор рисков оттока» на GitHub других репозиториев кейса не нашёл. Нашлись ещё проекты Invoice Keeper, но это другой кейс.

## Где мы сейчас (по `docs/load/`)

| Что | Наше число | Цель | Источник |
|---|---|---|---|
| Поток событий | 30–35 событий/с на 10 клиентах; каждое событие переписывает снимок целиком | ≥ 1 000/с на 100k [ПРЕДПОЛОЖЕНО] | `2026-10-24-events-2500.md`, `internal/source/stream.go` |
| Загрузка снимка 100k | 19–24 с в контейнере разработки, 42–45 с на раннере | ≤ 30 с | `2026-10-24-snapshot-stages.md`, `2026-10-23-load-100k.md` |
| Список клиентов 100k, 10 VU | p95 1,78 с, 145 RPS, 0 % ошибок | p95 < 1 с | `2026-10-23-load-100k.md` |
| Пересчёт 100k | 15,1 с на стенде; чистый расчёт ≈ 0,95 с | ≤ 1 с | `2026-10-23-load-100k.md`, #146 |

## Что из этого следует

1. Главный соперник — ChurnRadar: у них событийная архитектура и k8s-демо. Наш ответ — один процесс на Go, где все ядра работают с общей памятью, без сети между сервисами. Это нужно показать числами: поток на 100k клиентов, залп 10–50 тыс. событий, ничего не упало, согласованность до цента.
2. Без этих чисел поток у нас слабее заявленного у ChurnRadar (≈ 100 событий/с). Поэтому первым идёт `event-ingest-delta` (#155), потом `load-burst-proof` (#156) и `pod-kill-under-load` (#157).
3. Сверка с эталоном до цента на 300k (#153) — такого нет ни у кого из просмотренных конкурентов.
4. Чужие числа на слайдах не называем: мы их не проверяли.
