# Дизайн: Набор стенда: ручные аккаунты 1001+ для наглядного демо

## Подход

- `stand.json` в форме `data.json`, вшит через `embed`; `source.OnStart` склеивает его с набором организаторов при `SEED_STAND=true` (новый файл, `OnStart` вызывает функцию склейки одной строкой).
- Golden: `python3 -I tools/oracle/oracle.py` на склеенном наборе → `testdata/golden/stand.json`; Go-тест в `internal/risk` читает его так же, как `organizers.json`.
