# customers: изменения сценариев

## ADDED

### Requirement: фильтр «к биллингу»

`GET /api/v1/customers` SHALL принимать `billing=exclude` и `billing=only`; без параметра список прежний.

#### Scenario: исключить

- WHEN запрошено `billing=exclude` на данных организаторов
- THEN клиента 5 в списке нет, `total` = 9

#### Scenario: только биллинг

- WHEN запрошено `billing=only`
- THEN в списке только клиент 5, `total` = 1
