# prod-ingress: изменения сценариев

## ADDED

### Requirement: стенд отвечает стабильно

#### Scenario: 20 запросов подряд

- WHEN клиент 20 раз подряд открывает `https://nihao-churn.duckdns.org/healthz`
- THEN все 20 ответов `200` с действительным сертификатом
