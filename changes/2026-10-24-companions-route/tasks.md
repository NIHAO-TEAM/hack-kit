# Задачи: маршруты спутников на k3s

Каждая задача проверяема: в конце строки — чем проверить.

## 1. Диагностика

- [x] 1.1 `diagnose.sh` печатает endpoints, endpointslices спутников, версию и предупреждения Traefik — проверка: лог `deploy-ingress.yml` (ручной запуск) без секретов; причина записана в `design.md` (проверит CI: ручной запуск `deploy-ingress.yml`)

## 2. Чарт

- [x] 2.1 Спутники через `Endpoints` (или правка по найденной причине) — проверка: `helm template … | kubeconform` (job `lint` в `deploy-k8s.yml`) (проверит CI: helm в среде нет)

## 3. Прод

- [ ] 3.1 После выката `/grafana/` → `/grafana/login`, discovery Keycloak — JSON с `issuer` на домене — проверка: `deploy-smoke.yml`, тест «Grafana за входом» зелёный

## 4. Готовность

- [ ] 4.1 Сценарий `specs/k8s` про маршруты спутников
