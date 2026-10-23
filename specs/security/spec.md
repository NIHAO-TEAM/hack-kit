# security: безопасные заголовки

### Requirement: безопасные заголовки

Каждый ответ сервера SHALL содержать CSP без `unsafe-inline` для скриптов, HSTS, `X-Content-Type-Options`, `Referrer-Policy` и запрет встраивания во фрейм.

#### Scenario: ответ панели

- WHEN браузер открывает панель на проде
- THEN ответ содержит CSP, HSTS и запрет встраивания в чужой фрейм

Тесты: `backend/internal/platform/httpx/security_test.go`, `backend/internal/platform/docs/docs_test.go` (страница без встроенных скриптов).
