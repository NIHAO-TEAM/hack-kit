Статус: на ревью

# Прод: маршруты /grafana и /sso доходят до спутников

## Зачем

Шкала «Работоспособность» и «Архитектура»: мониторинг по ссылке и вход через Keycloak на проде. После выката `k3s-companions` (#96, прод `a19e859`) дымовая проверка красная (run 40043638096): `/grafana/` и `/sso/realms/churn/.well-known/openid-configuration` отдают страницу «Страница не найдена» фронта (575 байт `index.html`), то есть Traefik отправляет их в `/` приложения. Спутники при этом живы: `172.17.0.1:13000` (Grafana) и `:18080` (Keycloak) слушают (диагностика `deploy-ingress`, run 40043638060).

Гипотеза (не проверена на кластере): Traefik из k3s ищет адреса бэкенда через `Endpoints`, а чарт создаёт для Service без селектора только `EndpointSlice` (`templates/companions.yaml`). Нет адресов — Traefik отбрасывает правило `/grafana`, `/sso`, и срабатывает `/`. Ручной `Endpoints` зеркалится в `EndpointSlice` контроллером Kubernetes, обратного зеркалирования нет.

## Что меняется

- Диагностика: `deploy/k3s/diagnose.sh` печатает `kubectl -n churn get endpoints,endpointslices`, версию Traefik и его предупреждения про сервисы `churn-grafana`/`churn-keycloak` (без секретов).
- Чарт: спутники описаны `Endpoints` (адрес хоста, порт `http`) вместо `EndpointSlice`; если диагностика покажет другую причину — правка по ней, причина в «Найдено по ходу».
- Проверка после выката: `/grafana/` → 302 на `/grafana/login`, discovery Keycloak — JSON с `issuer` на домене.

Вне изменения:
- Compose-спутники и их настройки — нет.
- Дымовая проверка `e2e/` — нет.

## Затронутые части

- deploy: `deploy/helm/churn/templates/companions.yaml`, `deploy/k3s/diagnose.sh`
- контракт `api/openapi.yaml`: нет
- защищённые пути: да — `deploy/`; причина: маршрут спутников на проде
- зависит от: нет

## Связанные решения

- ADR: 0001
- Макет: нет

## Найдено по ходу

