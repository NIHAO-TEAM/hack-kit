Статус: черновик

# SSO на проде: ключи realm и OIDC дописывает выкат

## Зачем

После `keycloak-admin-env` (#103) Keycloak на проде проходит старт, но падает на импорте realm: «ERROR: Failed to start server in (production) mode / argument "content" is null» (лог `deploy-ingress.yml` на 9f16e61). В `/home/deploy/churn/.env` нет `KC_SECURE_PASSWORD`, `KC_SECURE_TOTP_SECRET`, `OIDC_CLIENT_SECRET` (предупреждения того же выката): плейсхолдеры `${…}` в `churn-realm.json` пустые. Keycloak на проде ни разу не стартовал («Initializing master realm» — база `kcdata` пустая), поэтому новые значения ничего не ломают. Блок SSO в `.env` создаёт только `bootstrap.sh` при первом запуске; на этом сервере `.env` создан без него.

## Что меняется

- `deploy/k3s/sso-env.sh` (новый): дописывает в `.env` недостающие или пустые ключи SSO — те же, что `bootstrap.sh`: `KC_ADMIN_PASSWORD`, `KC_SECURE_PASSWORD`, `KC_SECURE_TOTP_SECRET`, `OIDC_CLIENT_SECRET` (генерация на сервере, `openssl rand -hex 16`, `umask 077`), `OIDC_CLIENT_ID=panel`, `OIDC_ISSUER_URL=https://<домен>/sso/realms/churn`, `OIDC_DISCOVERY_URL=http://keycloak:8080/sso/realms/churn`, `OIDC_REDIRECT_URL=https://<домен>/api/v1/auth/oidc/callback`. Существующие непустые значения не трогает. В лог — только имена добавленных ключей.
- `deploy/k3s/deploy.sh` (шаг `deploy-k8s.yml`, идёт раньше `deploy-ingress.yml`): вызывает `sso-env.sh` до сборки values — приложение и Keycloak получают один и тот же `OIDC_CLIENT_SECRET` в одном выкате.
- `deploy/k3s/deploy.sh`: `KC_*` не попадают в Secret приложения (сейчас фильтр `_(PASSWORD|TOKEN)=` уносит туда `KC_ADMIN_PASSWORD` и `KC_SECURE_PASSWORD`).
- `deploy/k3s/single-ingress.sh`: предупреждения по ключам SSO остаются как сигнал; дописывание `KC_ADMIN_PASSWORD` из #103 заменяется вызовом `sso-env.sh`.

Вне изменения:
- Realm и поток OIDC в приложении — нет.
- `bootstrap.sh` — нет (по возможности переиспользует `sso-env.sh`, если это не меняет его поведение).

## Затронутые части

- deploy: `deploy/k3s/`
- контракт `api/openapi.yaml`: нет
- защищённые пути: да — `deploy/`; причина: SSO на проде не стартует без ключей realm
- зависит от: нет

## Связанные решения

- ADR: 0001 (OIDC — Keycloak, вход по паролю — запасной)
- Изменения: `keycloak-admin-env` (#103), `keycloak-prod-502` (#100), `sso-button-flag`
- Макет: нет

## Найдено по ходу

