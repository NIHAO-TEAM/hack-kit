Статус: на ревью

# Keycloak на проде: пароль админа из выката, ошибки старта видны в логе

## Зачем

Keycloak на проде в цикле перезапусков (код 2, 70+ рестартов), `/sso` отвечает 502, дымовой тест discovery из `keycloak-prod-502` красный. Причина установлена вручную на сервере: в `/home/deploy/churn/.env` нет `KC_ADMIN_PASSWORD` (SSO настраивали вручную, `bootstrap.sh` ключ не создал), Compose передаёт `KC_BOOTSTRAP_ADMIN_USERNAME=kcadmin` с пустым паролем, Keycloak 26 падает: «bootstrap-admin-username available only when bootstrap admin password is set». Эту строку диагностика выката вырезала фильтром секретов (`grep -v password|secret|token`), поэтому в логе `deploy-ingress.yml` причины не было.

## Что меняется

- `deploy/k3s/single-ingress.sh`: перед подъёмом спутников, если в `.env` нет непустого `KC_ADMIN_PASSWORD`, — сгенерировать на сервере (`openssl rand -hex 16`, `umask 077`), записать в `.env`, в лог — только «KC_ADMIN_PASSWORD добавлен в .env». Пустая строка `KC_ADMIN_PASSWORD=` заменяется (нынешний `set_env` пустые не трогает).
- `deploy/docker-compose.prod.yml`: `KC_BOOTSTRAP_ADMIN_USERNAME: ${KC_ADMIN_PASSWORD:+kcadmin}` — без пароля Keycloak стартует, а не падает (админ уже есть в `kcdata`).
- `deploy/k3s/single-ingress.sh`, `deploy/k3s/diagnose.sh`: строки лога Keycloak с `password|secret|token|totp` не вырезаются, а маскируются: всё после `=` или `: ` в такой строке — `***`. Сообщение об ошибке видно, значений нет.
- Проверка наличия ключей SSO в выкате (`KC_SECURE_PASSWORD`, `KC_SECURE_TOTP_SECRET`, `OIDC_CLIENT_SECRET`): нет ключа — `::warning::` с именем, без значения. Не генерируются: `OIDC_CLIENT_SECRET` должен совпадать с секретом приложения в k3s, остальные уже импортированы в realm.

Вне изменения:
- Realm, учётки, поток OIDC в приложении — нет.
- `.env` на сервере руками — нет, всё делает выкат.

## Затронутые части

- deploy: `deploy/k3s/single-ingress.sh`, `deploy/k3s/diagnose.sh`, `deploy/docker-compose.prod.yml`
- контракт `api/openapi.yaml`: нет
- защищённые пути: да — `deploy/`; причина: Keycloak на проде не стартует без `KC_ADMIN_PASSWORD`
- зависит от: нет

## Связанные решения

- ADR: 0001 (OIDC — Keycloak, вход по паролю — запасной)
- Изменения: `keycloak-prod-502` (диагностика и тест discovery), `k3s-companions`
- Макет: нет

## Найдено по ходу

