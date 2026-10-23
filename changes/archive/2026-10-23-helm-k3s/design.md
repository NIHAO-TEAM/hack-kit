# Дизайн: Прод на Kubernetes: Helm-чарт и k3s

## Подход

- Образ — тот же, что в Compose; данные Postgres переносятся `pg_dump`/`pg_restore` при переключении.
- Если k3s не поднимается за 2 часа работы — прод остаётся на Compose, чарт в репо проверен `helm lint`/`kubeconform`; запись в «Найдено по ходу».

## Решения по ходу

- **Образ.** Реестра нет: `deploy/k3s/deploy.sh` собирает образ на сервере (`docker build`) и импортирует в containerd (`k3s ctr images import`), `pullPolicy: IfNotPresent`. Нужен Docker (ставит `deploy/bootstrap.sh`) и sudo без пароля.
- **Миграции** — при старте приложения (`db.Up` в `main.go`), Job не нужен; Pod не Ready, пока миграция не прошла.
- **TLS** — ACME встроенного Traefik k3s (резолвер `le`, tls-alpn), без cert-manager: меньше компонентов. Хранилище сертификатов — том Traefik.
- **Секреты** — Secret создаётся чартом из временного values-файла (600), собранного на сервере из `~/churn/.env`; в логах значений нет, в репозитории нет.
- **Переключение** — разово `workflow_dispatch` с `cutover=true` → `deploy/k3s/cutover.sh`: дамп базы Compose, остановка (не удаление) стека, k3s, база в чарте при `replicaCount=0`, восстановление дампа, запуск приложения. Откат — `docker compose up -d` (тома целы).
- **Два деплоя.** `deploy.yml` (Compose) менять запрещено правилами. Автодеплой на k3s включается переменной `PROD_TARGET=k8s`; после переключения `deploy.yml` упирается в занятые 80/443 и краснеет — человеку стоит его отключить.
- [ПРЕДПОЛОЖЕНО] Версия k3s `v1.31.4+k3s1`, Traefik из k3s поддерживает `ports.web.redirections` и `persistence` в `HelmChartConfig`; в среде k3s и кластера нет, проверит прод.
- [ПРЕДПОЛОЖЕНО] Middleware заголовков безопасности — CRD `traefik.io/v1alpha1` (в k3s ≥1.30); `kubeconform` пропускает CRD (`-ignore-missing-schemas`).
