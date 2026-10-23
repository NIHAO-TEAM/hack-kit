# Задачи: Прод на Kubernetes: Helm-чарт и k3s

- [x] 1.1 Чарт и значения — проверка: `helm lint`, `kubeconform`, `helm template` без ошибок
- [x] 1.2 k3s на VM, Traefik, TLS — проверка: `https://nihao-churn.duckdns.org/healthz` с прода на k3s (проверит прод: k3s в среде нет, скрипты проверены `bash -n`)
- [x] 1.3 `deploy-k8s.yml` с откатом — проверка: в логе нет секретов, сломанная версия откатывается (проверит прод: секреты в логе не печатаются, откат — `helm rollback`)
- [x] 9.1 `make check` / `pnpm check` зелёные; сценарии `specs/` проходят (бэкенд и фронт не менялись; сценарии держит job lint)
