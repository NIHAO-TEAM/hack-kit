# Дизайн: cutover на k3s

## Подход

1. Убрать `2>/dev/null` у проверки в `cutover.sh` и печатать ошибку `compose ps` (без секретов); проверять наличие контейнера через `docker ps -a --filter label=com.docker.compose.project=churn --filter label=com.docker.compose.service=postgres -q`.
2. Запуск `deploy-k8s.yml` с `cutover=true`, разбор лога.
3. После переключения — `deploy-ingress.yml` (один вход) и проверка 20 подряд.

## Допущения

- [ГИПОТЕЗА] `compose ps` падает молча из-за другого пути `SRC` в `deploy-k8s.yml` или переменных окружения; подтвердить по логу шага 1.

## Решения по ходу

- [ПРЕДПОЛОЖЕНО] Причина «Postgres не найден»: `compose ps` разбирает весь `docker-compose.prod.yml`, а там обязательные переменные (`${…:?}`, например `GRAFANA_ADMIN_PASSWORD`), которых в старом `.env` может не быть; ошибка скрывалась `2>/dev/null`. Подтвердить по логу запуска. Исправление не зависит от причины: контейнер ищется по меткам Compose (`docker ps -a --filter label=com.docker.compose.project=churn`), дамп и остановка идут прямыми командами `docker`, без разбора compose-файла. При пустом поиске скрипт печатает список контейнеров.
- Пустой дамп прерывает cutover до остановки Compose.
