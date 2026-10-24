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

### Requirement: ресурсы прода вмещаются в узел

Лимиты памяти приложения и PostgreSQL SHALL быть заданы в Helm и Compose, а выкат SHALL проверять, что узел их вмещает; иначе выкат останавливается с «нужен человек».

#### Scenario: лимиты не вмещаются в узел

- WHEN сумма лимитов памяти чарта больше allocatable памяти узла
- THEN `deploy/k3s/deploy.sh` завершается с ошибкой «нужен человек» до выката

Держит: `deploy/k3s/deploy.sh` (проверка allocatable). Значения лимитов пока выбраны до замера; подбор по замеру пика на 100 000 клиентов — отдельное изменение.

### Requirement: бинарь собран по профилю и знает предел памяти

Образ приложения SHALL собираться с профилем `default.pgo`, а под — получать `GOMEMLIMIT` ниже лимита памяти контейнера.

#### Scenario: выкат

- WHEN выкачена новая версия
- THEN `go version -m` бинаря показывает сборку с PGO, в окружении пода есть `GOMEMLIMIT`

Держит CI: шаг `go version -m /out/api | grep -- "-pgo="` в `Dockerfile` (сборка образа падает без профиля). `GOMEMLIMIT` в `deploy/helm/churn/values.yaml` проверяется ревью: `helm template` его только рендерит.

### Requirement: убийство реплики под залпом без ошибок

Сервис SHALL работать в двух репликах так, что удаление одной из них и перезапуск выката во время залпа событий не дают ответов 5xx читателям и не нарушают согласованность данных.

#### Scenario: удаление пода под залпом

- WHEN идёт залп 10 000 событий и чтение очереди, удалён под реплики, не ведущей поток
- THEN 0 ответов 5xx у читателей, SSE переподключились, залп завершён, согласованность 0 центов
- Тест: `deploy-chaos.yml` (ручной, не гейт; скрипт `docs/load/chaos.sh`)

### Requirement: одновременный прогон потока только на одной реплике

Сервис SHALL не давать запустить поток событий, пока его ведёт другая реплика.

#### Scenario: запуск на второй реплике

- WHEN прогон потока идёт на одной реплике и приходит запуск на другую
- THEN ответ 409, прогон продолжается
- Тест: `TestStreamBusyOnOtherReplica` (`backend/internal/source/stream_http_test.go`)
