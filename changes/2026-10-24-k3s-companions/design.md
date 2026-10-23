# Дизайн: k3s-companions

## Подход

Вариант выбирает исполнитель по скорости и надёжности, порядок предпочтения:

1. **Спутники остаются в Compose, вход один — Traefik.** `single-ingress.sh` останавливает только `caddy` и `app`/`postgres` Compose, а `keycloak`, `grafana`, `prometheus` продолжают работать, публикуя порты только на адрес, доступный из k3s (не `0.0.0.0` наружу). В чарте — `Service` без селектора + `EndpointSlice` на адрес хоста и правила Ingress `/sso` → Keycloak, `/grafana` → Grafana. Prometheus забирает `/metrics` приложения через Ingress с `METRICS_TOKEN` или через NodePort. Быстро и обратимо, тома не трогаем.
2. **Спутники в чарте.** Deployment'ы Keycloak, Grafana, Prometheus в `deploy/helm/churn` (или отдельные чарты). Чище для критерия «Kubernetes», но дольше и требует переноса данных Keycloak; берём, только если вариант 1 упирается в сеть.

Порты 9090/3000/8080 не должны открыться наружу: проверка — `curl` с внешнего адреса к ним не проходит (`deploy/k3s/ingress-check.sh` или аналог в CI).

## Смоук: таблица очереди после входа

Гипотезы, проверять по порядку, причину записать сюда:

- [ГИПОТЕЗА] секрет `E2E_JURY_PASSWORD` не совпадает с `DEMO_JURY_PASSWORD` в `~/churn/.env` на проде или его нет в Secret релиза (`deploy/k3s/deploy.sh` берёт `*_PASSWORD`) — тогда вход 401 и таблица не появляется;
- [ГИПОТЕЗА] 5 с ожидания `toBeVisible` мало: первый запрос очереди в Firefox из раннера дольше;
- [ГИПОТЕЗА] после входа редирект не на `/queue`.

Чтобы видеть причину, а не гадать: при падении смоук загружает трассу Playwright артефактом (без пароля в логе). Пароль и ответы с данными в лог не печатаются.

## Допущения

- [ПРЕДПОЛОЖЕНО] на VM хватает памяти на Keycloak + Grafana + Prometheus рядом с k3s (до cutover они работали в Compose на той же VM).
- [ПРЕДПОЛОЖЕНО] если нужен секрет, которого нет в репозитории, исполнитель ставит `needs-human` и пишет, какой именно.

## Решения по ходу

- Выбран вариант 1: спутники остаются в Compose, вход один — Traefik. Keycloak, Grafana, Prometheus публикуют порты только на `COMPANIONS_BIND` (адрес docker0, `172.17.0.1`; по умолчанию `127.0.0.1`), снаружи закрыты. [ПРЕДПОЛОЖЕНО] адрес docker0 доступен из подов k3s.
- Чарт: `templates/companions.yaml` — Service без селектора + EndpointSlice на `companions.hostAddress`; Ingress `/sso` → Keycloak, `/grafana` → Grafana; админка и master-realm закрыты отдельным Ingress с `ipAllowList` (как в Caddy). [ПРЕДПОЛОЖЕНО] Traefik v3 (`ipAllowList`); на v2 нужен `ipWhiteList`.
- OIDC: `deploy.sh` переносит `OIDC_*` из `.env` в env и Secret; `OIDC_DISCOVERY_URL` из `http://keycloak:8080` подменяется на Service чарта `<релиз>-keycloak`. `.env` не меняется, откат на Compose работает.
- Prometheus: цель и схема из `METRICS_TARGET`/`METRICS_SCHEME` (по умолчанию `app:8080`/`http`); `single-ingress.sh` ставит `<домен>:443`/`https` — опрос через Traefik с bearer-токеном.
- Смоук: загрузка трассы Playwright требует правки существующего `deploy-smoke.yml`, а `executor.md` запрещает исполнителю менять существующие workflow. Задачи 1.1, 1.2, 3.1, 4.1 не выполнены. Новая гипотеза причины падения очереди: в k3s-Postgres нет данных (перенос базы вне изменения, Compose-база осталась в томе), поэтому таблица очереди пуста или не рисуется. [ГИПОТЕЗА] Проверить на проде: `diagnose.sh` и смоук после выката.
