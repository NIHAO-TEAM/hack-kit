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
