Статус: черновик

# Переключить прод на k3s (cutover)

## Зачем

Решение владельца продукта 23.10: «ставь» — прод на Kubernetes, как у aéza («кубер», `docs/experts/2026-10-23-aeza-chat.md`). Сейчас k3s на сервере не установлен, прод на Compose + Caddy; чарт `deploy/helm/churn` и скрипты `deploy/k3s/` уже в `main` (#65, #79).

## Что меняется

- Ручной запуск `deploy-k8s.yml` с `cutover=true` (23.10 20:40 UTC, run 40020634898) упал до изменений: `cutover.sh` сообщил «контейнера Postgres Compose нет, переносить нечего», хотя `docker ps` на сервере показывает `churn-postgres-1` (healthy). Найти причину (ошибка `compose ps` скрыта `2>/dev/null`: путь к compose-файлу, `--env-file`, пользователь, `HOME`) и исправить `cutover.sh`.
- Повторить cutover: дамп базы Compose → остановка Compose (тома остаются) → k3s и Traefik → чарт → восстановление дампа → приложение. Проверка: `https://nihao-churn.duckdns.org/healthz` 20 из 20, вход `jury` работает, данные те же (версия снимка, настройки модели).
- `deploy.yml` уже не выкатывает Compose при работающем k3s (916c988). После cutover выкат идёт через `deploy-k8s.yml`.
- Если cutover не удаётся — откат: `docker compose up -d` (тома целы), запись в «Найдено по ходу», прод остаётся на Compose.

Вне изменения:
- Prometheus, Grafana, Keycloak в кластере — их изменения.

## Затронутые части

- backend: нет
- frontend: нет
- контракт `api/openapi.yaml`: нет
- защищённые пути: да — `deploy/k3s/`, `.github/workflows/deploy*.yml` (разрешено, `executor.md`)
- зависит от: нет

## Связанные решения

- ADR: 0001, п. 6 (прод)
- Макет: не нужен

## Найдено по ходу

