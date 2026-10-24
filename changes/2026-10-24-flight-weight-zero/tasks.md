# Задачи: flight — вес 0

## 1. Модель

- [x] 1.1 Оракул: вес `flight` 0, golden перегенерирован — проверка: `python3 -I tools/oracle/oracle.py --check` — 0 расхождений
- [x] 1.2 `flight.DefaultWeight()` = 0, честный текст причины — проверка: `go test ./internal/risk ./internal/app` по новому golden
- [x] 1.3 Список новых контрольных чисел из оракула — в «Найдено по ходу» `proposal.md`

## 2. Тексты и числа

- [x] 2.1 Фронт: `featureInfo.flight` (название, подсказка, вес 0), «О модели», моки — проверка: `pnpm check`
- [x] 2.2 `docs/model.md` (таблица весов, чувствительность), `docs/model-rules.md`, `docs/demo-script.md`, `README.md`, `specs/*` — проверка: grep старых чисел пуст
- [x] 2.3 `e2e/tests/demo.spec.ts` — новые числа — проверка: e2e на локальном стенде — проверит CI (стенд в среде не поднять)

## 3. Готовность

- [x] 3.1 `make check`, `pnpm check`, oracle `--check` зелёные
- [x] 3.2 Прод: сохранённые настройки не держат `flight` 0,15 (проверить `GET /model/settings`) — в коде записи нет, смотри «Найдено по ходу»
- [x] 3.3 `specs/risk-model/spec.md` обновлён при архивации
