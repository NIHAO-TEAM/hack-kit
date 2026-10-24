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

### Requirement: смена настроек не перечитывает данные

Пересчёт после смены настроек модели SHALL использовать уже загруженный снимок той же версии и не читать таблицы источника заново.

#### Scenario: правка веса

- WHEN аналитик меняет вес признака, а версия снимка не менялась
- THEN пересчёт читает только версию снимка, очередь перестраивается, версия модели растёт
- Тест: `TestRecalcSkipsTableReadWhenSnapshotVersionSame`

#### Scenario: новые данные

- WHEN принят новый снимок или применено действие эмулятора
- THEN пересчёт читает таблицы заново и очередь показывает новые данные
- Тест: `TestRecalcSkipsTableReadWhenSnapshotVersionSame`

### Requirement: параллельный расчёт даёт те же числа

Параллельный расчёт SHALL давать результат, равный последовательному, до бита.

#### Scenario: сравнение

- WHEN один снимок посчитан последовательно и параллельно
- THEN клиенты, действия, сводка и проверки качества совпадают
- Тест: `TestScoreParallelEqualsSequential` (`internal/app/score_scale_test.go`)

### Requirement: пересчёт использует все ядра и не меняет результат

Пересчёт SHALL раздавать клиентов пачками пулу воркеров по числу ядер и давать результат, идентичный последовательному расчёту.

#### Scenario: результат не изменился

- WHEN пересчитаны данные организаторов и синтетический снимок на 10 000 клиентов
- THEN уровни, скоры и потери совпадают с последовательным расчётом до цента
- Тест: `TestPipelineEqualsSequential` (`internal/app/recalc_pipeline_test.go`), `TestScoreParallelEqualsSequential`

### Requirement: новый снимок отменяет текущий пересчёт

Новое событие снимка или настроек SHALL отменять идущий пересчёт; отменённый прогон не публикует результат.

#### Scenario: второй снимок во время пересчёта

- WHEN во время пересчёта приходит второй снимок
- THEN первый прогон отменён, опубликован результат по второму снимку, `churn_recalc_cancelled_total` вырос на 1
- Тест: `TestNewEventCancelsRunningRecalc` (`internal/analytics/engine_test.go`), `TestScoreContextCancel`, `TestRecalcMetrics` (`internal/analytics/recalc_metrics_test.go`)

#### Scenario: поток событий

- WHEN подряд приходит 2 500 событий
- THEN пересчёты не отменяются бесконечно: прогон после отменённого доходит до конца, результаты публикуются во время потока
- Тест: `TestEventStreamStillCompletes`

### Requirement: профиль и бенчмарки

Сервер SHALL давать `pprof` только по токену метрик, а репозиторий — бенчмарки пересчёта на 10 000 и 100 000 клиентов.

#### Scenario: профиль без токена

- WHEN запрос к служебному маршруту профиля без токена
- THEN ответ `401` или `403`
- Тест: `TestPprofOnlyWithMetricsToken` (`internal/auth/tokens_test.go`); бенчмарки `BenchmarkRecalc10k`, `BenchmarkRecalc100k` (`internal/app/recalc_bench_test.go`)

### Requirement: пересчёт не выделяет память на каждую услугу

Пересчёт SHALL переиспользовать буферы воркера между клиентами пачки и давать результат, побитово равный эталону.

#### Scenario: пересчёт 100 000 клиентов

- WHEN идёт `BenchmarkRecalc100k`
- THEN выделений на операцию меньше, чем до изменения (2,46 млн против 8,53 млн, `docs/load/2026-10-24-recalc-alloc.md`), golden и parity без расхождений
- Тест: бенчмарк `BenchmarkRecalc100k` (`internal/app/recalc_bench_test.go`), golden (`internal/risk/golden_test.go`), `TestParity10k` (`-tags parity`)
