# auth

### Requirement: вход по логину и паролю

Система SHALL выдавать сессию по верному логину и паролю без второго фактора.

#### Scenario: верный пароль

- WHEN клиент отправляет `POST /api/v1/auth/login` с верными логином и паролем
- THEN ответ `200`, в теле `token`, `csrfToken` и пользователь с ролью; установлена cookie `session` с флагом HttpOnly
- Тест: `TestLoginCorrectPassword`

#### Scenario: неверный пароль

- WHEN клиент отправляет неверный пароль
- THEN ответ `401` `problem+json`, без указания, что именно неверно, cookie не устанавливается
- Тест: `TestLoginWrongPassword`

#### Scenario: перебор пароля

- WHEN с одного IP для одного логина пришло 5 неверных попыток за 5 минут
- THEN шестая попытка получает `429` с заголовком `Retry-After`, даже с верным паролем
- Тест: `TestLoginRateLimit`

### Requirement: без входа данных нет

Система SHALL отвечать `401` на любой путь с данными клиентов или настройками без сессии.

#### Scenario: запрос без сессии

- WHEN клиент без cookie и без `Authorization` запрашивает `GET /api/v1/customers`
- THEN ответ `401` `problem+json`
- Тест: `TestNoSessionMeans401`

#### Scenario: открытые пути

- WHEN клиент без сессии запрашивает `GET /api/v1/docs`, `GET /api/v1/openapi.yml` или `GET /healthz`
- THEN ответ не `401`
- Тест: `TestPublicPathsOpen`

### Requirement: сессия по Bearer и по cookie

Система SHALL принимать токен сессии как `Authorization: Bearer` и как cookie; для cookie небезопасные методы требуют `X-CSRF-Token`.

#### Scenario: Bearer без CSRF

- WHEN клиент с `Authorization: Bearer <token>` роли `analyst` отправляет `PATCH /api/v1/model/settings`
- THEN запрос проходит проверку доступа (не `401` и не `403`)
- Тест: `TestCSRFOnlyForCookieSessions/Bearer_без_X-CSRF-Token_проходит`

#### Scenario: cookie без CSRF

- WHEN клиент с cookie сессии отправляет `PATCH /api/v1/model/settings` без `X-CSRF-Token`
- THEN ответ `403` `problem+json`
- Тест: `TestCSRFOnlyForCookieSessions/cookie_без_X-CSRF-Token_—_403`

### Requirement: права ролей на сервере

Система SHALL запрещать маркетологу менять модель и данные, в том числе через API.

#### Scenario: маркетолог меняет коэффициент

- WHEN пользователь роли `marketer` отправляет `PATCH /api/v1/model/settings`
- THEN ответ `403` `problem+json`, настройки не изменились
- Тест: `TestMarketerCannotChangeModelOrData; матрица — TestPermissionMatrixCoversEveryOperation`

#### Scenario: путь жюри

- WHEN пользователь `jury` входит по паролю и вызывает `PATCH /api/v1/model/settings`, `PUT /api/v1/source/snapshot`, `GET /api/v1/analytics/summary`
- THEN ни один вызов не получает `401` или `403`
- Тест: `TestJuryPath`

### Requirement: завершение сессии

#### Scenario: выход

- WHEN клиент вызывает `POST /api/v1/auth/logout`, затем `GET /api/v1/auth/me` с тем же токеном
- THEN второй ответ `401`
- Тест: `TestMeAndLogout`

### Requirement: служебные токены

#### Scenario: токен сборщика

- WHEN клиент с `Authorization: Bearer <METRICS_TOKEN>` запрашивает `GET /metrics`, а затем `GET /api/v1/customers`
- THEN первый ответ не `401`, второй — `401`
- Тест: `TestMetricsToken`

### Requirement: вход через Keycloak с TOTP

Система SHALL впускать пользователя Keycloak с паролем и кодом TOTP с ролью из claim и не впускать с неверным кодом.

#### Scenario: верный код

- WHEN пользователь `secure` нажимает «Войти через SSO», вводит пароль и текущий код TOTP
- THEN он возвращается в приложение с ролью из claim и видит свою рабочую страницу
- Тест: `e2e/tests/sso.spec.ts` (нужен Keycloak)

#### Scenario: неверный код

- WHEN код TOTP неверный
- THEN Keycloak показывает ошибку, сессии в приложении нет
- Тест: `e2e/tests/sso.spec.ts` (нужен Keycloak)

#### Scenario: SSO выключен

- WHEN `GET /api/v1/auth/oidc/login` отвечает `501`
- THEN на экране входа нет кнопки «Войти через SSO», вход по паролю работает
- Тест: `LoginPage.test.tsx`
