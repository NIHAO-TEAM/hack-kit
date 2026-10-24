# k8s: изменения сценариев

## ADDED

### Requirement: встраивание одним файлом values

Чарт SHALL встраиваться в чужой кластер правкой одного файла values: источник REST, OIDC, Prometheus.

#### Scenario: пример aéza

- WHEN `helm template churn deploy/helm/churn -f values-aeza.example.yaml`
- THEN манифесты проходят `kubeconform`, у приложения заданы `BILLING_URL` и `OIDC_ISSUER_URL`, есть ServiceMonitor, нет спутников и своего Postgres
