# Задачи: cutover на k3s

- [x] 1.1 Видимая ошибка и надёжная проверка Postgres Compose в `cutover.sh` — проверка: лог запуска показывает причину
- [x] 1.2 Cutover `deploy-k8s.yml cutover=true` — проверка: `kubectl get pods -n churn` Ready, дамп восстановлен
- [x] 1.3 Прод на k3s — проверка: 20 из 20 `https://nihao-churn.duckdns.org/healthz`, вход `jury`, `GET /api/v1/source/status` — та же версия снимка
