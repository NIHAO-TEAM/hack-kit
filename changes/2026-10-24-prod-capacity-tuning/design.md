# Дизайн: ресурсы прода

## Подход

1. Замер: стенд в раннере или контейнере, `cases/analizator/generate.py` на 10k/100k/300k, после пересчёта и во время потока 10 000 событий — `go_memstats_heap_inuse_bytes`, `process_resident_memory_bytes`, RSS PostgreSQL (`docker stats`). Таблица с железом.
2. Лимиты: приложение = пик RSS на 100k × 1,3 (300k — указать, влезает ли на прод-узел); PostgreSQL — `shared_buffers` + рабочая память.
3. Узел прода: шаг выката читает allocatable память узла; лимиты не превышают её с учётом Compose-компаньонов (Keycloak, Grafana, Prometheus). Не вмещается — выкат красный с «нужен человек», прод не трогается.
4. Пул: `pgxpool.ParseConfig` + `MaxConns` из `DB_MAX_CONNS`.

## Допущения

- [ПРЕДПОЛОЖЕНО] Размер прод-узла неизвестен пахану; шаг выката его узнаёт и печатает (без секретов).

## Риски

- Конфликт `values.yaml` с `go-pgo-build` (#160) и `pod-kill-under-load` — слиянием `main`.
