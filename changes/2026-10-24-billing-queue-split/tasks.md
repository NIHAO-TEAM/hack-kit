# Задачи: биллинг отдельно от очереди

## 1. Контракт и данные

- [x] 1.1 Параметр `billing` (`exclude`/`only`) в `api/openapi.yaml`, `make generate`, типы фронта — проверка: `make generate && git diff --exit-code`, `oasdiff` без breaking

## 2. Логика

- [x] 2.1 `CustomerCtx.Transactions`, новое условие `to_billing`, заголовок с числом попыток — проверка: тест `risk`: клиент 5 да; 10, 2, 3, 7 нет; не хватает денег — нет
- [x] 2.2 Фильтр `billing` в `ListCustomers` до сортировки и страниц — проверка: тест `analytics`: `exclude` без 5, `only` = [5], `total`
- [x] 2.3 `docs/model.md`: правило и ограничение — проверка: ревью

## 3. Экран

- [x] 3.1 Основная очередь `billing=exclude`, свёрнутая группа «У биллинга (N)» `billing=only` — проверка: тест очереди: клиент 5 не в таблице, есть в группе, группа свёрнута
- [x] 3.2 Состояния группы, живое обновление обоих списков — проверка: тест

## 4. Готовность

- [ ] 4.1 `make check`, `pnpm check`, oracle `--check` зелёные
- [ ] 4.2 Локальный стенд, данные организаторов: клиент 5 в группе, остальные в очереди
- [ ] 4.3 `specs/customers/spec.md`, `specs/queue-ui/spec.md`, `specs/retention-actions/spec.md` обновлены при архивации
