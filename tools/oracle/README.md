# Оракул модели

Эталонный расчёт модели риска оттока. Правила — `docs/model-rules.md`, числа — `testdata/golden/`. Нужен Python 3.10+, сторонних библиотек нет.

## Команды

```
python3 -I tools/oracle/oracle.py cases/analizator/data.json --as-of 2026-10-19T15:00:00Z   # расчёт в stdout
python3 -I tools/oracle/oracle.py --check   # пересчитать все golden и сравнить; код 1 при расхождении
python3 -I tools/oracle/oracle.py --regen   # перезаписать golden
```

`--settings <файл>` — переопределение настроек (`weights`, `high`, `medium`, `horizonHours`, `fundsHours`, `priceBasis`).

## Golden

`testdata/golden/*.json` (кроме `*.snapshot.json`) — по файлу на сценарий: `asOf`, `source` (снимок от корня репозитория), `settings`, результат. Граничные снимки лежат рядом: `edge-*.json` + `*.snapshot.json`. Новый сценарий: файл с `asOf`, `source`, `settings: {}`, затем `--regen`.

Пересчитывать golden (`--regen`) можно только вместе с правкой `docs/model-rules.md` и с объяснением в PR. Расхождение `--check` без такой правки — ошибка кода, а не golden.

## Как читает Go-тест (этап 3)

Для каждого файла `testdata/golden/*.json` без суффикса `.snapshot.json`: загрузить снимок из `source`, посчитать на `asOf` с `settings`, сравнить с `customers` и `summary` до цента; `score` — до 2 знаков; признаки — с допуском `1e-6`. `lost` и `unscored` имеют `score: null`, `level: null`.
