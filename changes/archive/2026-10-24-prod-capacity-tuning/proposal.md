Статус: на ревью

# Ресурсы прода под 100k и залп: лимиты памяти, PostgreSQL, пул соединений

## Зачем

Залп событий на 100 тыс. клиентов (ADR-0004, п. 3 и 6; #156) должен пройти и на проде, а не только в раннере. Проверено по `main`:
- В Helm у приложения и PostgreSQL `limits.memory: 512Mi` (`deploy/helm/churn/values.yaml`). Пересчёт 100k выделяет ≈ 843 МБ за прогон (`docs/load/2026-10-24-recalc-bench.md`). Сколько из этого живая куча, не замерено: гипотеза — на 100k и тем более 300k под упирается в лимит и получает OOMKill.
- PostgreSQL запущен с настройками по умолчанию (`shared_buffers` 128 МБ, `max_wal_size` 1 ГБ) — `COPY` 1,4 млн строк и поток пачек упираются в контрольные точки.
- Пул `pgxpool` с размером по умолчанию (`max(4, NumCPU)`): при залпе приём, пересчёт и чтение делят 4 соединения.

## Что меняется

- Замер живой кучи и RSS на 10k / 100k / 300k (после пересчёта и во время потока) — таблица в `docs/load/2026-10-24-capacity.md`; по ней лимиты приложения в Helm и Compose (запас 30 %), `requests` по факту.
- PostgreSQL: параметры через `args` в StatefulSet и `command` в Compose — `shared_buffers` 25 % лимита, `effective_cache_size`, `max_wal_size` 4 ГБ, `checkpoint_completion_target` 0,9, `wal_compression on`, `work_mem` по замеру; лимит памяти пода PostgreSQL — по замеру. `fsync` и `full_page_writes` не трогаем.
- Пул: `DB_MAX_CONNS` (по умолчанию 16) и `pool_max_conn_lifetime`; `max_connections` PostgreSQL с запасом.
- Проверка, что прод-узел вмещает новые лимиты (`kubectl describe node` в выкате: allocatable память) — если не вмещает, выкат пишет «нужен человек» и не меняет лимиты.

Вне изменения:
- Число реплик (это `pod-kill-under-load`), `GOMEMLIMIT` (это `go-pgo-build`: берёт значение из лимита, который ставит это изменение).

## Затронутые части

- backend: `internal/platform/db` (размер пула из окружения)
- frontend: нет
- контракт `api/openapi.yaml`: нет
- защищённые пути: `deploy/helm/churn/`, `deploy/docker-compose.prod.yml`, `docker-compose.yml` (лимиты и параметры PostgreSQL по причине выше)
- зависит от: нет

## Связанные решения

- ADR-0009: вертикальное масштабирование — процесс и БД должны получить ресурсы узла.
- Гейт не ослабляется.
