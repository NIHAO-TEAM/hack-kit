# api-contract: изменения сценариев

## ADDED

### Requirement: кривой ввод не даёт 5xx

Сервер SHALL отвечать на некорректные запросы к любой операции контракта кодом `4xx` в `application/problem+json` с `correlation_id` и никогда — `5xx`.

#### Scenario: строка вместо числа

- WHEN в параметр `limit` списка клиентов передана строка, а в тело `PATCH /model/settings` — отрицательный вес
- THEN ответ `400` или `422` в `problem+json`, не `500`
- Тест: проход по контракту в `internal/apitest`
