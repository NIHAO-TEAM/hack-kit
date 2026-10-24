# customers

### Requirement: рейтинг клиентов

#### Scenario: по умолчанию

- WHEN маркетолог запрашивает `GET /api/v1/customers` на данных организаторов
- THEN первые строки — клиенты 9 и 1 (3 189,10 € и 375,08 €), клиенты 2, 3, 7, 8 — в конце с состоянием «потерян»
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
- THEN в ответе услуги с признаками, весами и вкладами, главная причина — признак с наибольшим вкладом («Автопродление выключено, срок услуги уже вышел»), признак `funds` — «Нужно 1 444,90 € на 7 дней, на балансе 915,06 €», действие «Пополнить баланс на 529,84 €» с эффектом −1 027,49 €
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

### Requirement: фильтр «к биллингу»

`GET /api/v1/customers` SHALL принимать `billing=exclude` и `billing=only`; без параметра список прежний.

#### Scenario: исключить

- WHEN запрошено `billing=exclude` на данных организаторов
- THEN клиента 5 в списке нет, `total` = 9
- Тест: `TestListCustomersBillingFilter`

#### Scenario: только биллинг

- WHEN запрошено `billing=only`
- THEN в списке только клиент 5, `total` = 1
- Тест: `TestListCustomersBillingFilter`

### Requirement: выгрузка очереди в CSV

Сервис SHALL отдавать очередь целиком в CSV с теми же фильтрами и сортировкой, что список.

#### Scenario: набор организаторов

- WHEN `jury` скачивает `GET /api/v1/customers/export.csv` без параметров
- THEN 10 строк и заголовок, порядок — по ожидаемой потере, сумма колонки «Ожидаемая потеря, €» — 3 599,07
- Тест: `TestExportCustomersOrganizers`

#### Scenario: Excel

- WHEN файл открыт в Excel с русской локалью
- THEN кириллица читается, суммы — числа (UTF-8 с BOM, `;`, десятичная запятая)
- Тест: `TestExportCustomersOrganizers` (BOM, разделитель, запятая)

#### Scenario: фильтры

- WHEN заданы `level`, `billing` или `sort`/`order`
- THEN в файле только подходящие строки в нужном порядке; неверное значение — 400
- Тест: `TestExportCustomersFilters`

#### Scenario: журнал

- WHEN выгрузка выполнена
- THEN в журнале запись `customersExported` со строками и фильтрами, без email и текста поиска
- Тест: `TestExportCustomersJournalHasNoEmail`

#### Scenario: без входа

- WHEN запрос без сессии
- THEN 401, маршрут не путается с `/customers/{id}`
- Тест: cmd/api `TestExportRouteIsNotCustomerID` (нужна БД)


### Requirement: список клиентов читается из готовой модели

Список клиентов SHALL читаться из неизменяемой модели чтения, построенной после пересчёта и опубликованной атомарно; порядок, фильтры и страницы не отличаются от прежних.

#### Scenario: тот же порядок

- WHEN запрошен список с сортировкой по ожидаемой потере на данных организаторов
- THEN порядок и значения совпадают с прежним ответом, при равных значениях по возрастанию id
- Тест: `TestReadModelOrderMatchesReference` (все поля, оба направления, фильтр по уровню), существующие тесты `customers`

#### Scenario: чтение во время пересчёта

- WHEN идёт пересчёт, а клиенты запрашивают список
- THEN каждый ответ построен из одной целой версии модели (старой или новой)
- Тест: `TestReadersSeeWholeModelDuringSwap` с `-race`

### Requirement: пагинация по курсору

Список SHALL поддерживать необязательный `cursor` и отдавать `nextCursor`; курсор привязан к версии модели.

#### Scenario: обход без повторов

- WHEN список из 10 000 клиентов обойдён курсором страницами по 150
- THEN каждый клиент встретился ровно один раз, порядок как у страниц `offset`
- Тест: `TestCursorWalksAllWithoutGapsOrRepeats`

#### Scenario: устаревший курсор

- WHEN между страницами модель пересчитана
- THEN ответ `409`, клиент начинает обход заново; курсор вместе с `offset` или для другой сортировки — `400`
- Тест: `TestCursorStaleAndMisuse`
