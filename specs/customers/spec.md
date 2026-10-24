# customers

### Requirement: рейтинг клиентов

#### Scenario: по умолчанию

- WHEN маркетолог запрашивает `GET /api/v1/customers` на данных организаторов
- THEN первые строки — клиенты 9 и 1 (3 459,27 € и 383,15 €), клиенты 2, 3, 7, 8 — в конце с состоянием «потерян»
- Тест: `TestListCustomersDefault`, `TestCustomersAPI`

#### Scenario: сортировка в обе стороны

- WHEN запрошена сортировка по `id`, `email`, `balance` или `totalReplenished` с `order=asc` и `order=desc`
- THEN строки упорядочены по полю в указанном направлении, «нет оценки» и «потерян» внизу
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

### Requirement: стоимость потерянных услуг в строке клиента

Строка клиента SHALL содержать `lostCents` — стоимость услуг `deleted`/`blocked` за горизонт модели, в центах.

#### Scenario: клиент 7

- WHEN запрошен `GET /api/v1/customers/7` на данных организаторов
- THEN `customer.lostCents` равен значению оракула (680 874), а у клиента без потерянных услуг — 0
- Тест: `TestCustomerLostCents`
