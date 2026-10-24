# customers

### Requirement: рейтинг клиентов

#### Scenario: по умолчанию

- WHEN маркетолог запрашивает `GET /api/v1/customers` на данных организаторов
- THEN первые строки — клиенты 9 и 1 (3 459,27 € и 383,15 €), клиенты 2, 3, 7, 8 — в конце с состоянием «потерян»
- Тест: `TestListCustomersDefault`, `TestCustomersAPI`

#### Scenario: сортировка в обе стороны

- WHEN запрошена сортировка по `id`, `email`, `balance` или `totalReplenished` с `order=asc` и `order=desc`
- THEN весь список, включая «нет оценки» и «потерян», упорядочен по полю в указанном направлении; при равенстве — по ID
- AND на данных организаторов `sort=id&order=asc` даёт 1, 2, …, 10, а `order=desc` — 10, 9, …, 1
- Тест: `TestListCustomersSorts`

#### Scenario: сортировка по риску

- WHEN запрошена сортировка по `score` или `expectedLoss` (или сортировка не указана)
- THEN оценённые клиенты упорядочены по полю, «нет оценки» и «потерян» — внизу
- Тест: `TestListCustomersSorts`, `TestListCustomersDefault`

#### Scenario: фильтр по уровню

- WHEN запрошено `level=lost&sort=id&order=desc` на данных организаторов
- THEN порядок 8, 7, 3, 2
- Тест: `TestListCustomersSorts`

### Requirement: карточка клиента

#### Scenario: причины и деньги

- WHEN запрошен `GET /api/v1/customers/9`
- THEN в ответе услуги с признаками, весами и вкладами, главная причина — признак с наибольшим вкладом («Автопродление выключено, срок услуги уже вышел»), признак `funds` — «Нужно 1 444,90 € на 7 дней, на балансе 915,06 €», действие «Пополнить баланс на 529,84 €» с эффектом −1 067,48 €
- Тест: `TestGetCustomer9`

#### Scenario: нет такого клиента

- WHEN запрошен несуществующий ID
- THEN ответ `404` `problem+json`
- Тест: `TestGetCustomerLostAndMissing`
