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
- THEN в ответе услуги с признаками, весами и вкладами, причина «не хватает 529,84 € на 7 дней» и действие «пополнить» с эффектом −1 067,48 €
- Тест: `TestGetCustomer9`

#### Scenario: нет такого клиента

- WHEN запрошен несуществующий ID
- THEN ответ `404` `problem+json`
- Тест: `TestGetCustomerLostAndMissing`
