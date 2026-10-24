# customers: изменения сценариев

## ADDED

### Requirement: стоимость потерянных услуг в строке клиента

Строка клиента SHALL содержать `lostCents` — стоимость услуг `deleted`/`blocked` за горизонт модели, в центах.

#### Scenario: клиент 7

- WHEN запрошен `GET /api/v1/customers/7` на данных организаторов
- THEN `customer.lostCents` равен значению оракула
- AND у клиента без потерянных услуг `lostCents` = 0
