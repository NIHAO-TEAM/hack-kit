# k8s: прод на k3s по Helm-чарту

### Requirement: прод на Kubernetes

#### Scenario: выкат

- WHEN слит PR и CI зелёный
- THEN новая версия работает на k3s по тому же домену с TLS; база недоступна снаружи кластера

Держат: `helm lint`, `helm template` и `kubeconform` в `.github/workflows/deploy-k8s.yml` (job `lint`); NetworkPolicy `churn-postgres` в чарте; на проде — проверка `/healthz` в `deploy/k3s/deploy.sh`.

#### Scenario: сломанная версия

- WHEN новая версия не Ready или не отвечает на `/healthz`
- THEN `helm --atomic` или `helm rollback` возвращает прошлую ревизию, деплой отмечен красным

### Requirement: прод на k3s без потери данных

#### Scenario: после переключения

- WHEN прод переключён на k3s (`deploy-k8s.yml`, `cutover=true`)
- THEN по тому же домену с TLS открывается стенд, данные и настройки модели те же, что до переключения (проверка: `deploy/k3s/deploy.sh` ждёт `/healthz`, откат `helm rollback`; сквозная проверка вручную)

### Requirement: спутники за единым входом

Прод на k3s SHALL отдавать Grafana по `/grafana` и Keycloak по `/sso` через Traefik.

#### Scenario: Grafana по ссылке

- WHEN без входа открыт `https://<домен>/grafana/`
- THEN перенаправление на `/grafana/login`, форма входа Grafana

#### Scenario: Keycloak discovery

- WHEN запрошен `https://<домен>/sso/realms/churn/.well-known/openid-configuration`
- THEN JSON с `issuer` = `https://<домен>/sso/realms/churn`

Держат: `Endpoints` спутников в `deploy/helm/churn/templates/companions.yaml`, `deploy-smoke.yml` (тест «Grafana за входом»).

### Requirement: встраивание одним файлом values

Чарт SHALL встраиваться в чужой кластер правкой одного файла values: источник REST, OIDC, Prometheus.

#### Scenario: пример aéza

- WHEN `helm template churn deploy/helm/churn -f values-aeza.example.yaml`
- THEN манифесты проходят `kubeconform`, у приложения заданы `BILLING_URL` и `OIDC_ISSUER_URL`, есть ServiceMonitor, нет спутников и своего Postgres

Держит: `.github/workflows/deploy-k8s-values.yml`.

### Requirement: ресурсы прода вмещают портфель 100k под потоком

Лимиты памяти приложения и PostgreSQL SHALL задаваться по замеру пика на 100 000 клиентов под потоком событий с запасом 30 %, а выкат SHALL проверять, что узел их вмещает.

#### Scenario: поток на 100k на проде

- WHEN на проде портфель 100 000 клиентов и идёт поток 10 000 событий
- THEN под приложения не перезапускается (нет OOMKill)
Держат: замер в `docs/load/2026-10-24-capacity.md` (пока не выполнен) и `restartCount` после `Deploy k8s`.
