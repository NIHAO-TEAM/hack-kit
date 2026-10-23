# stand-data

### Requirement: наглядный стенд

#### Scenario: свежий prolong_wait

- WHEN стенд запущен с `SEED_STAND=true`
- THEN у клиента 1001 отсчёт «до удаления 2 д 07 ч», числа клиентов 1–10 те же, что на наборе организаторов
- Тест: `TestMergeStand`, `TestGolden/stand.json` (golden от оракула), `TestReaderLoadsStandSeed` (интеграционный)
