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

### Requirement: скор клиента по худшей услуге

Модель SHALL считать скор клиента как 100 · наибольшую вероятность непродления среди его живых услуг.

#### Scenario: две услуги

- WHEN у клиента две живые услуги с вероятностями 0,30 и 0,62
- THEN скор клиента 62, уровень «средний»
- Тест: `TestScoreTwoServices` (`backend/internal/risk/score_test.go`)

#### Scenario: пороги уровней

- WHEN скор 70 или выше
- THEN уровень «высокий»; от 50 до 70 — «средний»; ниже 50 — «низкий»
- Тест: `TestScoreThresholds`

### Requirement: признак добавляется одним файлом

#### Scenario: новый признак

- WHEN в пакет добавлен файл с новым признаком и его регистрацией
- THEN его вес появляется в настройках модели по умолчанию, а вклад — в причинах услуг, без правки других файлов
- Тест: `TestRegistryFeatureAppearsInDefaults`

### Requirement: ожидание оплаты

#### Scenario: до удаления 2 дня

- WHEN услуга в `prolong_wait`, до удаления (`expiresAt` + 5 дней) осталось 2 дня
- THEN значение признака `pwait` равно 0,6
- Тест: `TestPwait`

#### Scenario: срок ожидания вышел

- WHEN услуга в `prolong_wait`, а `expiresAt` + 5 дней уже прошло
- THEN `pwait` равно 1
- Тест: `TestPwait`

### Requirement: монотонность

#### Scenario: рост веса

- WHEN вес любого признака увеличен
- THEN скор ни одного клиента не уменьшился
- Тест: `TestScoreProperties`

### Requirement: нехватка средств на ближайшие продления

Модель SHALL учитывать, хватает ли на балансе денег на продления в ближайшие 7 дней, считая бонусный баланс только при `bonusState = unlocked`.

#### Scenario: почасовая услуга съест баланс

- WHEN у клиента почасовая услуга, и за 7 дней её списания больше баланса
- THEN признак «не хватает средств» положительный, причина называет нужную сумму и сумму на балансе
- Тест: `backend/internal/risk`, `TestFunds`

#### Scenario: бонус заблокирован

- WHEN бонусный баланс покрыл бы продления, но `bonusState = locked`
- THEN бонус не учитывается
- Тест: `backend/internal/risk`, `TestFundsBonusLocked`

### Requirement: качество данных видно, но не меняет скор

#### Scenario: устаревший статус

- WHEN услуга в `prolong_wait`, а с `expiresAt` прошло больше 5 дней
- THEN у услуги отметка «ошибка данных: в ожидании оплаты дольше 5 дней», а в сводке счётчик таких услуг
- Тест: `backend/internal/risk`, `TestQualityChecks, TestQualityOrganizers`

### Requirement: совпадение с эталоном

#### Scenario: набор организаторов

- WHEN модель считает набор организаторов при `asOf = 2026-10-19T15:00:00Z`
- THEN скор, уровень, признаки и деньги каждого клиента совпадают с `testdata/golden/organizers.json`
- Тест: `backend/internal/risk`, `TestGolden/organizers.json`
