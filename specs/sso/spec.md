# sso

### Requirement: вход через SSO

Система SHALL давать вход через OIDC-провайдера (Keycloak с ключом доступа и TOTP) второй кнопкой; вход по паролю остаётся. Сессия после SSO та же, что после пароля.

#### Scenario: ключ доступа

- WHEN пользователь нажимает «Войти через SSO» и подтверждает вход ключом доступа
- THEN открывается панель с его ролью; вход по паролю по-прежнему работает
- Тест: `TestSSOLoginCreatesSessionWithRole`, `TestSSOPasswordLoginStillWorks`, `LoginPage.test.tsx` (кнопка); сам ключ доступа — вручную на стенде с профилем `sso`

#### Scenario: PKCE, state и nonce

- WHEN клиент открывает `GET /api/v1/auth/oidc/login`
- THEN ответ `302` на провайдера с `code_challenge` (S256), `state`, `nonce`; состояние потока — в HttpOnly-cookie
- Тест: `TestSSOLoginCreatesSessionWithRole`

#### Scenario: подмена потока

- WHEN в callback пришёл чужой state, нет cookie потока, плохой код или чужой nonce
- THEN отказ (`400` или `401`), сессия не создаётся
- Тест: `TestSSORejects`, `TestSSOWrongNonceRejected`

#### Scenario: нет роли панели

- WHEN в ID-токене нет роли `marketer` или `analyst` (в том числе только `admin`, `jury`)
- THEN `403`, сессия не создаётся
- Тест: `TestSSORoleRequired`

#### Scenario: SSO-учётка без пароля

- WHEN вошедший через SSO пользователь `sso:<имя>` пробует войти по паролю
- THEN `401`
- Тест: `TestSSOSSOAccountHasNoPassword`

#### Scenario: SSO не настроен

- WHEN `OIDC_ISSUER_URL` пуст
- THEN `/auth/oidc/login` отвечает `501`
- Тест: `TestSSODisabledWithoutConfig`

#### Scenario: админка Keycloak закрыта

- WHEN снаружи открывают `/sso/admin` или `/sso/realms/master`
- THEN `404` (Caddy)
- Тест: проверяется на стенде после выката (в среде исполнителя нет Docker)

### Requirement: кнопка SSO по конфигурации

Экран входа SHALL показывать вход через SSO только когда SSO включён.

#### Scenario: стенд без Keycloak

- WHEN `OIDC_*` не заданы и пользователь открывает экран входа
- THEN `GET /api/v1/auth/config` отвечает `{ssoEnabled: false}` без входа, кнопки «Войти через SSO» нет, вход по паролю работает
- Тест: `TestAuthConfigWithoutOIDC`, `LoginPage.test.tsx` («SSO выключен», «сбой запроса настроек»)

#### Scenario: SSO настроен

- WHEN `OIDC_*` заданы
- THEN `ssoEnabled: true`, кнопка есть
- Тест: `TestAuthConfigWithOIDC`, `LoginPage.test.tsx` («SSO включён»)

### Requirement: Keycloak на проде

Прод SHALL отдавать discovery Keycloak по `/sso` после каждого выката.

#### Scenario: discovery

- WHEN запрошен `https://<домен>/sso/realms/churn/.well-known/openid-configuration`
- THEN 200, JSON с `issuer` = `https://<домен>/sso/realms/churn`
- Тест: `e2e/tests/smoke.spec.ts` («Keycloak: discovery на /sso»); выкат ждёт ответа до 180 с (`single-ingress.sh`)

### Requirement: старт Keycloak без пароля админа в .env

Выкат SHALL поднимать Keycloak, даже если в `.env` нет `KC_ADMIN_PASSWORD`.

#### Scenario: ключа нет

- WHEN в `.env` на сервере нет `KC_ADMIN_PASSWORD` (или он пустой) и запущен `deploy-ingress.yml`
- THEN ключ дописан в `.env` без значения в логе, Keycloak отвечает discovery 200
- Тест: прогон `single-ingress.sh` на копии `.env` (вручную); discovery — `e2e/tests/smoke.spec.ts`

#### Scenario: ошибка старта видна

- WHEN Keycloak падает на старте с сообщением, где есть слово `password`
- THEN текст ошибки есть в логе выката, значения переменных заменены на `***`
- Тест: проверка `sed` на строках ошибки и `KEY=value` (вручную, см. design.md)

### Requirement: ключи SSO из выката

Выкат SHALL дописывать недостающие ключи SSO в `.env` сервера до старта приложения и Keycloak.

#### Scenario: сервер без блока SSO

- WHEN в `.env` нет `KC_SECURE_PASSWORD`, `KC_SECURE_TOTP_SECRET`, `OIDC_CLIENT_SECRET` и `OIDC_*`, запущен выкат
- THEN ключи дописаны без значений в логе, у приложения и в realm один `OIDC_CLIENT_SECRET`, discovery `/sso` отвечает 200
- Тест: прогон `deploy/k3s/sso-env.sh` на копии `.env` (вручную); discovery — `e2e/tests/smoke.spec.ts`

#### Scenario: ключи уже есть

- WHEN все ключи SSO заданы
- THEN `.env` не меняется
- Тест: повторный прогон `deploy/k3s/sso-env.sh` на той же копии (вручную, `cmp`)
