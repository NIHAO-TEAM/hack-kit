# Задачи: Прод на Kubernetes: Helm-чарт и k3s

- [x] 1.1 Чарт и значения — проверка: `helm lint`, `kubeconform`, `helm template` без ошибок
- [ ] 1.2 k3s на VM, Traefik, TLS — проверка: `https://nihao-churn.duckdns.org/healthz` с прода на k3s
- [ ] 1.3 `deploy-k8s.yml` с откатом — проверка: в логе нет секретов, сломанная версия откатывается
- [ ] 9.1 `make check` / `pnpm check` зелёные; сценарии `specs/` проходят
