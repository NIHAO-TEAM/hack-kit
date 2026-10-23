# audit

### Requirement: кто и что поменял

Система SHALL записывать в журнал вход, выход, неудачный вход, изменения настроек модели, загрузку снимка, правки услуги и счёта и действия эмулятора; в записях нет паролей, токенов и email клиентов.

#### Scenario: изменение веса

- WHEN аналитик сохранил новый вес
- THEN в журнале запись: кто, когда, вес было → стало
- Тест: `TestAuditLog` (`internal/app/audit_integration_test.go`), `TestDiff` (`internal/model/diff_test.go`), экран: `src/app/audit-log.test.tsx`

#### Scenario: предпросмотр не пишется

- WHEN аналитик отправил `PATCH /api/v1/model/settings?dryRun=true`
- THEN записи в журнале нет
- Тест: `TestAuditLog`

#### Scenario: неудачный вход

- WHEN вход с неверным паролем
- THEN запись `loginFailed` с введённым логином, пароль в журнал не попадает
- Тест: `TestAuditLog`

### Requirement: чтение журнала

Система SHALL отдавать `GET /api/v1/audit` с пагинацией (`limit` 1..200, `offset`, `{items, total}`), новые записи сверху, аналитику, jury и администратору.

#### Scenario: права

- WHEN маркетолог запрашивает журнал
- THEN ответ 403; без сессии — 401
- Тест: `TestAuditPermissions`, матрица `TestPermissionMatrixCoversEveryOperation`
