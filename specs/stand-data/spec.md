# stand-data

### Requirement: наглядный стенд

#### Scenario: свежий prolong_wait

- WHEN стенд запущен с `SEED_STAND=true`
- THEN у клиента 1001 отсчёт «до удаления 2 д 07 ч», числа клиентов 1–10 те же, что на наборе организаторов
- Тест: `TestMergeStand`, `TestGolden/stand.json` (golden от оракула), `TestReaderLoadsStandSeed` (интеграционный)

### Requirement: сид стенда проходит проверку снимка

Сид стенда (`SEED_STAND=true`) SHALL проходить ту же проверку, что снимок источника, и сервер SHALL стартовать с ним.

#### Scenario: старт со стендом

- WHEN сервер стартует с `SEED_STAND=true` на пустой базе
- THEN сид записывается в БД без нарушения ограничений (в том числе `transactions_amount_check`), в очереди есть ручные клиенты 1001+
- Тест: `TestStandSeedValidates` (юнит, склейка сида проходит `Validate`), `TestReaderLoadsStandSeed` (интеграционный)
