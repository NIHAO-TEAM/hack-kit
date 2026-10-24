# source-ui

### Requirement: аналитик меняет данные

Система SHALL давать аналитику и `jury` видеть состояние источника (версия снимка, `asOf`, когда загружен, откуда), проверки качества, править услугу и счёт, двигать эмулятор и загружать файл снимка; маркетологу экран отвечает «Недостаточно прав».

#### Scenario: правка баланса

- WHEN аналитик увеличивает баланс клиента 9 на 529,84 €
- THEN ожидаемая потеря клиента 9 падает на 1 027,49 €, очередь у маркетолога обновилась
- Тест: `TestPatchAccountRecalculatesLoss` (`backend/internal/app/customers_integration_test.go`), `TestPatchAccount` (`backend/internal/source/patch_test.go`)

#### Scenario: плохой файл

- WHEN загружен снимок с услугой чужого аккаунта
- THEN ошибка по полю человеческим текстом, данные прежние
- Тест: `плохой файл: ошибка по полю человеческим текстом` (`frontend/src/app/source-screen.test.tsx`), сервер — `http_test.go`

#### Scenario: маркетолог

- WHEN маркетолог открывает «Источник»
- THEN «Недостаточно прав», запись данных на сервере отвечает 403
- Тест: `маркетолог видит «Недостаточно прав»`, `TestPatchAccountRecalculatesLoss`

### Requirement: проверки качества данных видны аналитику

Система SHALL показывать на экране «Источник» каждую проверку качества данных с результатом человеческим текстом и услугами, на которых она сработала.

#### Scenario: застарелое ожидание оплаты

- WHEN в снимке услуга в `prolong_wait` дольше 5 дней после срока
- THEN `GET /source/status` содержит проверку `stale_status` с `ok=false`, текстом и этой услугой; экран показывает текст и ссылку на клиента
- Тест: `TestStatusQualityChecks` (`backend/internal/app/quality_integration_test.go`), `source-screen.test.tsx` «проверки качества из сервера: текст, услуги и ссылки на клиентов» (`frontend/src/app/source-screen.test.tsx`), `TestChecksResult` (`backend/internal/risk/quality_test.go`)

#### Scenario: противоречий нет

- WHEN ни одна проверка не сработала
- THEN все проверки `ok=true`, экран пишет «Противоречий не найдено» без цвета тревоги
- Тест: «все проверки прошли — «Противоречий не найдено»» (`frontend/src/app/source-screen.test.tsx`)
