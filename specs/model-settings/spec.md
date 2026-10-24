# model-settings

### Requirement: изменение коэффициента

Система SHALL позволять аналитику читать и менять настройки модели; изменение сохраняется новой версией и запускает пересчёт.

#### Scenario: сохранить

- WHEN аналитик отправляет `PATCH /api/v1/model/settings` с весом `noauto = 0.85`
- THEN настройки сохранены с новой версией, модель пересчитана, клиент 1 стал высоким риском
- Тест: `TestSettingsAPI` (`internal/app/settings_integration_test.go`), `TestApplyMergesWeights` (`internal/model/settings_test.go`)

#### Scenario: предпросмотр

- WHEN тот же запрос отправлен с `dryRun=true`
- THEN ответ показывает, сколько клиентов сменит уровень и на сколько изменится ожидаемая потеря; настройки не изменились
- Тест: `TestSettingsAPI`, `TestDiff` (`internal/app/settings_test.go`)

#### Scenario: неверное значение

- WHEN вес вне [0, 1], высокий порог не больше среднего, горизонт вне 1..365 или признак неизвестен
- THEN ответ `422` с ошибкой по полю, настройки прежние
- Тест: `TestSettingsAPI`, `TestProblems` (`internal/model/settings_test.go`)

#### Scenario: перезапуск

- WHEN настройки сохранены, а приложение перезапущено
- THEN `GET /api/v1/model/settings` отдаёт сохранённые настройки, версию, автора и время
- Тест: `TestSettingsAPI`

### Requirement: чувствительность очереди к весам

Система SHALL по запросу аналитика или `jury` показать, как меняются уровни клиентов, первая тройка по потере и суммарная потеря при сдвиге веса каждого признака с ненулевым весом на −20 % и +20 %, не меняя сохранённые настройки.

#### Scenario: набор организаторов

- WHEN аналитик открывает «О модели»
- THEN виден блок «Насколько устойчива очередь» с числом сменивших уровень и пометкой про первую тройку по каждому признаку и каждому сдвигу
- Тест: `TestSensitivityGolden` (`internal/risk/sensitivity_golden_test.go`), `аналитику показывает, насколько устойчива очередь` (`frontend/src/app/about-model.test.tsx`)

#### Scenario: настройки не менялись

- WHEN запрошена чувствительность
- THEN сохранённые веса и версия настроек остались прежними
- Тест: `TestShiftWeightKeepsOriginalAndClamps` (`internal/risk/sensitivity_test.go`); вызов только читает, в БД не пишет

#### Scenario: кэш по версиям

- WHEN чувствительность запрошена повторно при тех же версиях снимка и настроек
- THEN расчёт не повторяется; одновременные запросы ждут один расчёт; дольше 10 секунд — `503`, расчёт доходит до конца и попадает в кэш
- Тест: `TestSensitivityCacheByVersions`, `TestSensitivityCacheSingleFlight`, `TestSensitivityCacheTimeoutKeepsCalculating` (`internal/model/sensitivity_test.go`)

#### Scenario: большой объём

- WHEN клиентов больше 20 000
- THEN расчёт идёт на детерминированной выборке 5 000 (по хэшу id), ответ помечен `sampled: true`, экран говорит, что числа относятся к выборке
- Тест: `TestSampleSnapshot` (`internal/risk/sensitivity_test.go`), `при расчёте на выборке объясняет…` (`about-model.test.tsx`)

#### Scenario: права

- WHEN маркетолог вызывает чувствительность
- THEN ответ `403`; без сессии — `401`; аналитик, `jury` и администратор получают `200`
- Тест: `TestModelSensitivityAccess` (`internal/auth/access_test.go`), `TestPermissionMatrixCoversEveryOperation`
