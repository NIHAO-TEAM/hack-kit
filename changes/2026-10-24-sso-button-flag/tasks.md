# Задачи: кнопка SSO по флагу

Каждая задача проверяема: в конце строки — чем проверить.

## 1. Контракт

- [ ] 1.1 `GET /api/v1/auth/config` в `api/openapi.yaml` — проверка: `make generate && git diff --exit-code`

## 2. Логика

- [ ] 2.1 `ssoEnabled` по конфигурации OIDC — проверка: тест `internal/auth`: без `OIDC_*` — `false`, с ними — `true`; ответ без входа 200

## 3. Экран

- [ ] 3.1 Кнопка SSO только при `ssoEnabled: true` — проверка: тест `LoginPage`: `false` и ошибка запроса — кнопки нет, `true` — есть

## 4. Готовность

- [ ] 4.1 `make check` и `pnpm check` зелёные
- [ ] 4.2 Сценарий `specs/sso` про кнопку при выключенном SSO
