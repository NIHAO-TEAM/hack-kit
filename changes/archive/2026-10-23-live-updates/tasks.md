# Задачи: живое обновление

- [x] 1.1 `GetEvents` (SSE, пульс, без буфера) — проверка: после `Publish("recalculated", 5)` клиент получает `event: recalculated` с `5`
- [x] 2.1 `app/realtime`: подписка, сброс ключей, состояние связи, переподключение — проверка: unit-тест с фейковым `EventSource`
- [x] 2.2 `useChangedRows` — проверка: unit-тест
- [ ] 2.3 Плашка «нет связи» — проверка: скриншоты обеих тем (плашка собрана на Alert из системы; скриншоты не сняты в среде — снимет ревью или стенд)
- [x] 3.1 `make check`, `pnpm check` зелёные; сценарии `specs/` проходят (go test, golangci-lint, pnpm typecheck/lint/lint:fsd/test зелёные; полный pnpm check и тесты с БД — CI)
