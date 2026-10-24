# stand-data: изменения сценариев

## ADDED

### Requirement: сид стенда проходит проверку снимка

Сид стенда (`SEED_STAND=true`) SHALL проходить ту же проверку, что снимок источника, и сервер SHALL стартовать с ним.

#### Scenario: старт со стендом

- WHEN сервер стартует с `SEED_STAND=true` на пустой базе
- THEN сид записывается в БД без нарушения ограничений (в том числе `transactions_amount_check`), в очереди есть ручные клиенты 1001+
- Тест: `TestStandSeedValidates` (юнит), `TestReaderLoadsStandSeed` (интеграционный)
