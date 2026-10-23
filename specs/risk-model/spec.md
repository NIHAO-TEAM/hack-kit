# risk-model

### Requirement: эталон расчёта на наборе организаторов

Система SHALL на наборе организаторов с `asOf = 2026-10-19T15:00:00Z` и настройками по умолчанию давать эти числа.

#### Scenario: уровни и деньги

- WHEN модель считает `cases/analizator/data.json` при `asOf = 2026-10-19T15:00:00Z`
- THEN высокий риск — клиенты 10, 5, 9; средний — 4, 1; низкий — 6; потеряны — 2, 3, 7, 8
- AND ожидаемая потеря 3 879,73 €, под угрозой 6 900,46 €, уже потеряно 7 368,34 €
- Тест: `oracle.py --check`, `testdata/golden/organizers.json`
#### Scenario: действие с ценой

- WHEN для клиента 1 применено действие «включить автопродление»
- THEN скор клиента 1 становится 41,57 (низкий), ожидаемая потеря снижается на 365,29 €
- Тест: `oracle.py --check`, `testdata/golden/organizers.json`


### Requirement: состояния без оценки

#### Scenario: клиент без услуг

- WHEN у аккаунта нет ни одной услуги
- THEN его состояние «нет оценки», скора нет
- Тест: `oracle.py --check`, `testdata/golden/edge-no-services.json`
#### Scenario: клиент ушёл

- WHEN все услуги аккаунта `deleted` или `blocked`
- THEN его состояние «потерян», скора нет, сумма «уже потеряно» больше нуля
- Тест: `oracle.py --check`, `testdata/golden/edge-gone-only.json`


### Requirement: правила записаны

#### Scenario: эталон воспроизводим

- WHEN оракул пересчитывает все golden-файлы
- THEN результат совпадает с файлами до цента
- Тест: `oracle.py --check`, все `testdata/golden/*.json`
