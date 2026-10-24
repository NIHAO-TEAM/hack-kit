# Задачи: ресурсы прода

Каждая задача проверяема: в конце строки — чем проверить.

## 1. Замер

- [ ] 1.1 Живая куча, RSS приложения и PostgreSQL на 10k/100k/300k, после пересчёта и под потоком — проверка: таблица в `docs/load/2026-10-24-capacity.md`

## 2. Настройки

- [ ] 2.1 Лимиты и `requests` приложения и PostgreSQL в Helm и Compose по замеру — проверка: `helm template`
- [ ] 2.2 Параметры PostgreSQL (`shared_buffers`, `max_wal_size`, `checkpoint_completion_target`, `wal_compression`, `effective_cache_size`) — проверка: `SHOW` в поде после выката
- [x] 2.3 `DB_MAX_CONNS` и `max_connections` — проверка: тест конфигурации пула
- [ ] 2.4 Шаг выката: allocatable память узла, «нужен человек», если не вмещается — проверка: лог `Deploy k8s`

## 3. Готовность

- [ ] 3.1 Загрузка снимка 100k до и после настроек PostgreSQL — проверка: строка в отчёте
- [ ] 3.2 `main` после слияния: CI, `Deploy k8s`, ingress, smoke зелёные, под не перезапускался — проверка: логи прогонов
