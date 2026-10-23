# k8s: прод на k3s по Helm-чарту

### Requirement: прод на Kubernetes

#### Scenario: выкат

- WHEN слит PR и CI зелёный
- THEN новая версия работает на k3s по тому же домену с TLS; база недоступна снаружи кластера

Держат: `helm lint`, `helm template` и `kubeconform` в `.github/workflows/deploy-k8s.yml` (job `lint`); NetworkPolicy `churn-postgres` в чарте; на проде — проверка `/healthz` в `deploy/k3s/deploy.sh`.

#### Scenario: сломанная версия

- WHEN новая версия не Ready или не отвечает на `/healthz`
- THEN `helm --atomic` или `helm rollback` возвращает прошлую ревизию, деплой отмечен красным
