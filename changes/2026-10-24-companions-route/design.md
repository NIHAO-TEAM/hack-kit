# Дизайн: маршруты спутников на k3s

## Подход

1. Сначала факт: ручной запуск `deploy-ingress.yml` (диагностика) или `diagnose.sh` в выкате — что видит Traefik: `kubectl -n churn get endpoints churn-grafana churn-keycloak`, `kubectl -n kube-system logs deploy/traefik | grep -i -E "grafana|keycloak|endpoint"`, версия образа Traefik.
2. Правка чарта: на каждый спутник `Service` (без селектора, порт `http` 8080 → `targetPort` спутника) + `Endpoints` с тем же именем, `subsets: [{addresses: [{ip: hostAddress}], ports: [{name: http, port: <порт спутника>}]}]`. `EndpointSlice` убрать (его создаст зеркалирование).
3. `helm template` и `kubeconform` в `deploy-k8s.yml` (lint на PR) зелёные.
4. После слияния выкат идёт сам (`PROD_TARGET=k8s`); исполнитель проверяет `/grafana/` и discovery Keycloak `curl` с раннера через `deploy-ingress.yml` или ручным запуском дымовой проверки.

## Допущения

- [ПРЕДПОЛОЖЕНО] Причина — отсутствие `Endpoints` (см. `proposal.md`). Если диагностика показывает другое (NetworkPolicy, `hostAddress` недоступен из пода, Traefik не видит namespace) — чинить найденное, гипотезу вычеркнуть.

## Риски

- Ошибка в чарте ломает выкат: `deploy.sh` с `--atomic` откатит релиз, прод останется на прошлой ревизии.
