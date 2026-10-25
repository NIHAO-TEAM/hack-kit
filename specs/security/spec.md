# security: безопасные заголовки

### Requirement: безопасные заголовки

Каждый ответ сервера SHALL содержать CSP без `unsafe-inline` для скриптов, HSTS, `X-Content-Type-Options`, `Referrer-Policy` и запрет встраивания во фрейм.

#### Scenario: ответ панели

- WHEN браузер открывает панель на проде
- THEN ответ содержит CSP, HSTS и запрет встраивания в чужой фрейм

Тесты: `backend/internal/platform/httpx/security_test.go`, `backend/internal/platform/docs/docs_test.go` (страница без встроенных скриптов).

### Requirement: гость только смотрит

#### Scenario: вход гостя

- WHEN вход гостя включён и человек нажал «Посмотреть как гость»
- THEN он вошёл с ролью `viewer` без пароля и видит очередь

#### Scenario: вход гостя выключен

- WHEN `GUEST_LOGIN_ENABLED` не задан
- THEN кнопки гостя на экране входа нет, `POST /auth/guest` отвечает 404

#### Scenario: гость не меняет данные

- WHEN гость вызывает любую пишущую операцию (задачи, веса модели, снимок, события, эмулятор, правка услуги или аккаунта) — из интерфейса или API
- THEN сервер отвечает 403, данные не меняются

#### Scenario: гость не видит служебное

- WHEN гость запрашивает журнал, чувствительность модели или поток эмулятора
- THEN сервер отвечает 403; в меню нет «Источника» и «Журнала»

#### Scenario: много гостей

- WHEN с одного адреса запросов входа гостя больше лимита
- THEN ответ 429 с `Retry-After`

Тесты: `backend/internal/auth/guest_test.go`, `backend/internal/auth/permissions_test.go`, `frontend/src/pages/login/ui/LoginPage.test.tsx`, `frontend/src/entities/session/model/rights.test.ts`.
