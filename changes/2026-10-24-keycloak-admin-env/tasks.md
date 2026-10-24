# Задачи: пароль админа Keycloak из выката

Каждая задача проверяема: в конце строки — чем проверить.

## 1. Выкат

- [x] 1.1 `single-ingress.sh`: дописать непустой `KC_ADMIN_PASSWORD`, если нет (пустую строку заменить) — проверка: `bash -n`, `shellcheck`, прогон на копии `.env` без ключа и с `KC_ADMIN_PASSWORD=`
- [ ] 1.2 `docker-compose.prod.yml`: `KC_BOOTSTRAP_ADMIN_USERNAME: ${KC_ADMIN_PASSWORD:+kcadmin}` — проверка: `docker compose config` с паролем и без
- [x] 1.3 Предупреждение по именам `KC_SECURE_PASSWORD`, `KC_SECURE_TOTP_SECRET`, `OIDC_CLIENT_SECRET` без значений — проверка: прогон на копии `.env`

## 2. Диагностика

- [ ] 2.1 Маскировка вместо вырезания в `single-ingress.sh` и `diagnose.sh` — проверка: строка ошибки видна, `KEY=value` → `KEY=***`

## 3. Готовность

- [ ] 3.1 После слияния: `deploy-smoke.yml` зелёный, тест discovery проходит — проверка: лог прогона
- [ ] 3.2 Сценарий `specs/sso` про старт без пароля админа
