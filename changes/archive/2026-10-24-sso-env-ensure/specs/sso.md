# sso: изменения сценариев

## ADDED

### Requirement: ключи SSO из выката

Выкат SHALL дописывать недостающие ключи SSO в `.env` сервера до старта приложения и Keycloak.

#### Scenario: сервер без блока SSO

- WHEN в `.env` нет `KC_SECURE_PASSWORD`, `KC_SECURE_TOTP_SECRET`, `OIDC_CLIENT_SECRET` и `OIDC_*`, запущен выкат
- THEN ключи дописаны без значений в логе, у приложения и в realm один `OIDC_CLIENT_SECRET`, discovery `/sso` отвечает 200

#### Scenario: ключи уже есть

- WHEN все ключи SSO заданы
- THEN `.env` не меняется
