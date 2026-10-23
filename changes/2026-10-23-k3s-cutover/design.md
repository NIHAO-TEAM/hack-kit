# Дизайн: cutover на k3s

## Подход

1. Убрать `2>/dev/null` у проверки в `cutover.sh` и печатать ошибку `compose ps` (без секретов); проверять наличие контейнера через `docker ps -a --filter label=com.docker.compose.project=churn --filter label=com.docker.compose.service=postgres -q`.
2. Запуск `deploy-k8s.yml` с `cutover=true`, разбор лога.
3. После переключения — `deploy-ingress.yml` (один вход) и проверка 20 подряд.

## Допущения

- [ГИПОТЕЗА] `compose ps` падает молча из-за другого пути `SRC` в `deploy-k8s.yml` или переменных окружения; подтвердить по логу шага 1.
