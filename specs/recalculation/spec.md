# recalculation

### Requirement: пересчёт по событию

Система SHALL пересчитывать модель после загрузки снимка и после изменения настроек, без участия пользователя.

#### Scenario: загружен новый снимок

- WHEN загружен новый снимок данных
- THEN через короткое время доступен результат с новой версией данных и публикуется сигнал «пересчитано» с номером версии
- Тест: `TestRecalcAtStartAndOnEvents` (`internal/analytics/engine_test.go`), `TestRaceSettingsAndSnapshot` (`internal/app/recalc_integration_test.go`)

#### Scenario: изменены настройки

- WHEN сохранены новые веса модели
- THEN результат пересчитан с новыми весами, номер версии вырос
- Тест: `TestRecalcAtStartAndOnEvents`, `TestStoreEmptyThenSave` (`internal/model/store_integration_test.go`)

#### Scenario: много изменений подряд

- WHEN за секунду пришло 10 изменений
- THEN последний результат соответствует последним данным и настройкам; лишних пересчётов в очереди нет
- Тест: `TestBurstOfEventsMakesOneExtraRecalc`

### Requirement: настройки сохраняются

#### Scenario: перезапуск

- WHEN настройки сохранены, а приложение перезапущено
- THEN модель считает с сохранёнными настройками
- Тест: `TestSettingsSurviveRestart`

#### Scenario: первый запуск

- WHEN настройки ни разу не сохранялись
- THEN модель считает с настройками по умолчанию
- Тест: `TestStoreEmptyThenSave`, `TestStartOnSeedMatchesScore`

### Requirement: сбой источника не стирает оценку

#### Scenario: ошибка чтения данных

- WHEN чтение снимка из базы завершилось ошибкой
- THEN остаётся предыдущий результат, ошибка записана в лог
- Тест: `TestErrorKeepsPreviousResult`
