# Дизайн: пароль админа Keycloak из выката

## Подход

1. Ключ в `.env` дописывает `single-ingress.sh` до `up -d … keycloak`: значение генерируется на сервере и не попадает ни в лог job, ни в репозиторий. Файл `.env` правится от пользователя выката (`PROD_USER`), владелец и права не меняются.
2. `${KC_ADMIN_PASSWORD:+kcadmin}` — подстановка Compose: пусто, если пароля нет. Второй рубеж на случай ручного `docker compose up` без выката.
3. Маскировка: `sed -E '/password|secret|token|totp/I s/(=|: )[^ ]+/\1***/g'` — проверка на строке ошибки «bootstrap-admin-username available only when bootstrap admin password is set» (текст остаётся) и на строке `KEY=value` (значение — `***`).
4. Проверка: выкат зелёный, `deploy-smoke.yml` — тест «Keycloak: discovery на /sso» проходит.

## Допущения

- [ПРЕДПОЛОЖЕНО] Админ master-realm уже создан в `kcdata` при первом старте: новый `KC_ADMIN_PASSWORD` его пароль не меняет, нужен только для запуска.

## Риски

- Нет для приложения: вход по паролю работает и без Keycloak.
- [ПРЕДПОЛОЖЕНО] Compose при пустом `KC_ADMIN_PASSWORD` передаёт `KC_BOOTSTRAP_ADMIN_USERNAME=""` (проверено `docker compose config`); Keycloak 26 считает пустую переменную незаданной — проверит `deploy-smoke.yml` после выката.
- Маскировка уточнена: `sed -E 's/((password|secret|token|totp)[A-Za-z_.-]*(=|: ))[^ ]+/\1***/gI'` — маскирует значение только у ключа с таким словом; строка `ERROR: bootstrap-admin-username available only when bootstrap admin password is set` остаётся целой (общий `(=|: )` из первой версии съел бы её после `ERROR: `).
