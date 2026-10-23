# Задачи: проверки качества данных на экране источника

## 1. Контракт и данные

- [x] 1.1 `QualityCheck.items` (необязательный массив `{serviceId, accountId}`) в `api/openapi.yaml` — проверка: `make generate && git diff --exit-code` после генерации, `oasdiff` без поломок

## 2. Логика

- [ ] 2.1 `GetSourceStatus` заполняет `checks` по реестру проверок — проверка: тест `TestStatusQualityChecks` (снимок с застарелым `prolong_wait` → `stale_status` `ok=false`, счётчик и услуга в `items`)
- [ ] 2.2 Набор организаторов: `checks` совпадают с `qualityCounts` сводки — проверка: тест на сиде

## 3. Экран

- [ ] 3.1 `QualityChecks.tsx` по `checks`, словарь кодов удалён, ссылки на карточку клиента — проверка: `pnpm check`, тест `source-screen.test.tsx` «проверки качества из сервера»

## 4. Готовность

- [ ] 4.1 `make check` и `pnpm check` зелёные
- [ ] 4.2 Сценарии `specs/source-ui` проходят на локальном стенде
