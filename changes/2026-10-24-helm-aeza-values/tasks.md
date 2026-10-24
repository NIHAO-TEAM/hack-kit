# Задачи: values для встраивания

Каждая задача проверяема: в конце строки — чем проверить.

## 1. Чарт

- [x] 1.1 `values-aeza.example.yaml` (REST-источник, OIDC, Prometheus, внешняя база) — проверка: `helm template … -f values-aeza.example.yaml | kubeconform -strict` (проверит CI: helm в среде нет)
- [x] 1.2 Пояснения в `values.yaml` — проверка: `helm lint` (job `lint` в `deploy-k8s.yml` на PR)

## 2. Документы

- [x] 2.1 Раздел «Встраивание в контур aéza» — команда установки и список секретов без значений — проверка: чтение ревьюером

## 3. Готовность

- [x] 3.1 Проверка примера в CI — новый workflow или запись человеку в «Найдено по ходу» — новый workflow deploy-k8s-values.yml
- [ ] 3.2 Сценарий `specs/k8s` про пример встраивания
