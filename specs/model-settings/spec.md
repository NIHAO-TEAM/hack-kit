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
