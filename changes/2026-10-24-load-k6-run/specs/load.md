# load: изменения сценариев

## ADDED

### Requirement: числа нагрузки из реального прогона

Система SHALL публиковать в `docs/load/` результаты прогона k6 со ссылкой на запуск workflow.

#### Scenario: прогон на раннере

- WHEN запущен `deploy-load.yml`
- THEN summary запуска содержит p95 и долю ошибок для `snapshot`, `recalc`, `read`
- AND прод не получает запросов на запись
