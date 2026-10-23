Статус: в работе

# Прод на Kubernetes: Helm-чарт и k3s

## Зачем

Эксперт aéza: деплой у них — «кубер»; «проверять все – от системного дизайна до деплоя в прод» (`docs/experts/2026-10-23-aeza-chat.md`). Тезис защиты: встраивание в контур — правка `values.yaml`.

## Что меняется

- `deploy/helm/churn`: Deployment приложения (readiness/liveness, ресурсы, не от root, read-only fs), Postgres (StatefulSet или внешний), миграции — при старте или Job, Ingress с TLS, NetworkPolicy (база доступна только приложению), Secret из значений, `ServiceMonitor` — выключен по умолчанию.
- k3s на проде (`deploy/k3s/bootstrap.sh`), Traefik и cert-manager (или ACME Traefik); прод переключается с Compose на чарт без смены домена; Compose остаётся для запуска по README.
- `.github/workflows/deploy-k8s.yml` — `helm upgrade --install` после зелёного CI; откат `helm rollback` при неудачной проверке `/healthz`.
- CI: `helm lint` и `kubeconform` для чарта.

Вне изменения:
- Grafana — этап 7.

## Затронутые части

- backend: нет
- frontend: нет
- контракт `api/openapi.yaml`: добавления в этом PR, если нужны (без поломки)
- защищённые пути: да — `deploy/`, `.github/workflows/deploy-k8s.yml` (разрешено, `executor.md`)
- зависит от: `prod-compose`

## Связанные решения

- ADR: 0001, п. 6
- Макет: не нужен

## Найдено по ходу

