# sso: изменения сценариев

## ADDED

### Requirement: старт Keycloak без пароля админа в .env

Выкат SHALL поднимать Keycloak, даже если в `.env` нет `KC_ADMIN_PASSWORD`.

#### Scenario: ключа нет

- WHEN в `.env` на сервере нет `KC_ADMIN_PASSWORD` (или он пустой) и запущен `deploy-ingress.yml`
- THEN ключ дописан в `.env` без значения в логе, Keycloak отвечает discovery 200

#### Scenario: ошибка старта видна

- WHEN Keycloak падает на старте с сообщением, где есть слово `password`
- THEN текст ошибки есть в логе выката, значения переменных заменены на `***`
