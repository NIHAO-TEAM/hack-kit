# auth

### Requirement: вход через Keycloak с TOTP

Система SHALL впускать пользователя Keycloak с паролем и кодом TOTP с ролью из claim и не впускать с неверным кодом.

#### Scenario: верный код

- WHEN пользователь `secure` нажимает «Войти через SSO», вводит пароль и текущий код TOTP
- THEN он возвращается в приложение с ролью из claim и видит свою рабочую страницу
- Тест: `e2e/tests/sso.spec.ts`

#### Scenario: неверный код

- WHEN код TOTP неверный
- THEN Keycloak показывает ошибку, сессии в приложении нет
- Тест: `e2e/tests/sso.spec.ts`

#### Scenario: SSO выключен

- WHEN `GET /api/v1/auth/oidc/login` отвечает `501`
- THEN на экране входа нет кнопки «Войти через SSO», вход по паролю работает
