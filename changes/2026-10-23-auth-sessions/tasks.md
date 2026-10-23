# Задачи: вход, сессии, права

Каждая задача проверяема: в конце строки — чем проверить.

## 1. Контракт и данные

- [x] 1.1 Запросы `sqlc` модуля `auth`: пользователи, сессии — проверка: `make generate && git diff --exit-code`

## 2. Логика

- [x] 2.1 Хеширование argon2id и проверка пароля — проверка: unit-тест (верный, неверный, пустой пароль)
- [x] 2.2 Учётки из окружения при старте (`ADMIN_*`, `DEMO_*_PASSWORD`) — проверка: интеграционный тест «без пароля учётка не создаётся; смена переменной меняет пароль»
- [x] 2.3 `login` / `me` / `logout`: cookie + Bearer, `csrfToken` — проверка: тесты через `apitest` по сценариям `specs/auth.md`
- [x] 2.4 Middleware: Bearer или cookie, CSRF только для cookie — проверка: тест «PATCH с cookie без `X-CSRF-Token` → 403; с Bearer без него → не 403»
- [ ] 2.5 Матрица прав `can(role, operation)` и проверка на каждой операции — проверка: табличный тест по всем `operationId` спеки × 4 роли
- [ ] 2.6 Лимит попыток входа — проверка: тест «6-я неудача за 5 минут → 429 с `Retry-After`»
- [ ] 2.7 Токены `SOURCE_TOKEN` и `METRICS_TOKEN` — проверка: тест «токен сборщика не открывает `/customers`»
- [ ] 2.8 Путь жюри: `login(jury) → PATCH /model/settings → PUT /source/snapshot → GET /analytics/summary` проходит авторизацию (тела пока 501) — проверка: тест `TestJuryPath`

## 3. Экран

- нет

## 4. Готовность

- [ ] 4.1 `make check` и `make test-int` зелёные
- [ ] 4.2 Сценарии из `specs/` проходят на локальном стенде
- [ ] 4.3 README и `.env.example` не трогать: их ведёт параллельное `compose-stand`, переменные `ADMIN_*`, `DEMO_*_PASSWORD`, `METRICS_TOKEN`, `SOURCE_TOKEN`, `COOKIE_SECURE`, `TRUSTED_PROXY` там уже перечислены. Новая переменная — запись в «Найдено по ходу»
