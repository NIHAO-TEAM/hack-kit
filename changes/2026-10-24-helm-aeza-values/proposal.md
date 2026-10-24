Статус: в работе

# Встраивание в контур aéza — один файл values

## Зачем

Шкала «Архитектура и расширяемость» (9): «Прод на Kubernetes по Helm-чарту из репо; встраивание в контур aéza — правка `values.yaml` (источник REST, OIDC, Prometheus)». Сейчас в `deploy/helm/churn/values.yaml` нет ни одного ключа про источник REST (`BILLING_URL`, `BILLING_POLL_SECONDS`, `BILLING_TOKEN` из `billing-rest-adapter`), OIDC (`OIDC_*`) и внешний Prometheus: они собираются из `~/churn/.env` скриптом `deploy/k3s/deploy.sh`. Жюри, читая чарт, не увидит, что именно правится при встраивании.

## Что меняется

- `deploy/helm/churn/values-aeza.example.yaml`: пример встраивания — свой домен и TLS-секрет, источник REST (адрес и интервал в `env`, токен — в `existingSecret`), OIDC внешнего Keycloak/IdP (`OIDC_ISSUER_URL`, `OIDC_CLIENT_ID`, `OIDC_REDIRECT_URL`; секрет клиента — в `existingSecret`), `serviceMonitor.enabled: true`, `companions.enabled: false`, `postgres.enabled: false` с `externalUrl`.
- `values.yaml`: закомментированные ключи с пояснением, где что задаётся.
- Раздел «Встраивание в контур aéza» в `deploy/helm/README.md` (или README чарта): команда `helm upgrade --install … -f values-aeza.example.yaml`, список секретов без значений.
- CI (`deploy-k8s.yml`, job `lint`): `helm template` с этим файлом и `kubeconform` — правка существующего workflow запрещена исполнителю, поэтому проверка — отдельный шаг в новом `deploy-k8s-values.yml` или задача человеку в «Найдено по ходу».

Вне изменения:
- Выкат прода (`deploy/k3s/deploy.sh`) — нет.
- Код приложения — нет.

## Затронутые части

- deploy: `deploy/helm/churn/`
- контракт `api/openapi.yaml`: нет
- защищённые пути: да — `deploy/`; причина: пример встраивания чарта
- зависит от: нет

## Связанные решения

- ADR: 0001
- Макет: нет

## Найдено по ходу

