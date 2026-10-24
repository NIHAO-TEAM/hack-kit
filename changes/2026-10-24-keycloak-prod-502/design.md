# Дизайн: Keycloak на проде

## Подход

1. Факт до правки: ручной запуск `deploy-ingress.yml` (в нём `diagnose.sh`) с новыми строками диагностики. Причину — в этот файл, раздел «Причина».
2. Правка по причине:
   - не запущен (профиль `sso` выключен) — `single-ingress.sh` поднимает Keycloak всегда, когда в `.env` есть `OIDC_ISSUER_URL`;
   - падает — по логу (память, база `kcdata`, импорт realm);
   - не тот адрес/путь — порт на `COMPANIONS_BIND`, `KC_HTTP_RELATIVE_PATH=/sso`, `KC_PROXY_HEADERS=xforwarded`, `KC_HOSTNAME=https://<домен>/sso`.
3. Ожидание готовности в выкате: до 180 с ждать 200 от discovery с хоста, иначе лог Keycloak в job и красный шаг (прод не откатывать: приложение работает, вход по паролю — запасной).
4. Тест в `smoke.spec.ts`: `request.get('/sso/realms/churn/.well-known/openid-configuration')` → 200, `issuer` = `${baseURL}/sso/realms/churn`.

## Допущения

- [ПРЕДПОЛОЖЕНО] Keycloak остаётся в Compose рядом с k3s (решение `k3s-companions`), в кластер не переносится.

## Причина

(заполняет исполнитель по диагностике)

## Риски

- Keycloak 26 на маленьком сервере стартует 1–2 минуты: проверка ждёт, а не падает сразу.
