# Задачи: оболочка фронта

Каждая задача проверяема: в конце строки — чем проверить.

## 1. Контракт и данные

- [x] 1.1 Приложение Vite + React + TS strict, зависимости по `frontend/AGENTS.md` с точными версиями — проверка: `pnpm install --frozen-lockfile && pnpm typecheck`
- [x] 1.2 `pnpm api:types` → `src/shared/api/schema.gen.ts` из `../api/openapi.yaml` — проверка: `pnpm api:types && git diff --exit-code`
- [x] 1.3 MSW-моки для `auth/*`, `customers`, `customers/{id}`, `analytics/summary`, `model/settings` на данных организаторов — проверка: `pnpm dev:mock`, вход `marketer`/`demo` открывает оболочку

## 2. Логика

- [x] 2.1 `entities/session`: `me`, `login`, `logout`, `can(role, right)`; клиент API с cookie и `X-CSRF-Token` — проверка: unit-тесты `can` по матрице ролей; тест клиента «CSRF-заголовок на PATCH»
- [x] 2.2 Охрана маршрутов: без сессии → `/login`; раздел без права → «Недостаточно прав» — проверка: тесты роутера (Testing Library)

## 3. Экран

- [ ] 3.1 Экран входа: поля, ошибка неверного пароля и «слишком много попыток» человеческим текстом — проверка: тест + скриншоты светлой и тёмной темы
- [ ] 3.2 Оболочка: навигация по роли, имя, выход; заглушки страниц с `shared/ui/states` — проверка: скриншоты на 1440 и 820 по `frontend-design-review`
- [ ] 3.3 Витрина `/dev/ui` с пятью состояниями — проверка: страница открывается в обеих темах

## 4. Готовность

- [ ] 4.1 `pnpm check` зелёный, `pnpm build` создаёт `frontend/dist`
- [ ] 4.2 Сценарии из `specs/` проходят на моках
- [ ] 4.3 `frontend/CHECKLIST.md` отмечен
