# Дизайн: values для встраивания

## Подход

1. Имена переменных — из кода (`backend/cmd`, `internal/source`, `internal/auth`): `BILLING_URL`, `BILLING_POLL_SECONDS`, `BILLING_TOKEN`, `OIDC_ISSUER_URL`, `OIDC_DISCOVERY_URL`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`, `OIDC_REDIRECT_URL`, `METRICS_TOKEN`. Несекретные — в `env`, секретные — перечислены как ключи `existingSecret` без значений.
2. Пример проходит `helm template churn deploy/helm/churn -f values-aeza.example.yaml | kubeconform -strict` локально и в CI.
3. Адреса — заглушки домена `example.aeza.internal`.

## Допущения

- [ПРЕДПОЛОЖЕНО] Prometheus aéza — prometheus-operator (ServiceMonitor); иначе пример аннотаций `prometheus.io/scrape` в комментарии.

## Риски

- Нет: прод выкатывается своими значениями, пример его не трогает.
