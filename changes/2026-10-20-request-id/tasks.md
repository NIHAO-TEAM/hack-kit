# Задачи: X-Request-Id

## 1. Логика

- [ ] 1.1 Добавить зависимость `github.com/google/uuid` (выбрать актуальную версию, сверить API по Context7) — проверка: `go build ./...`
- [ ] 1.2 Промежуточный слой `withRequestID` — проверка: `go vet ./...`
- [ ] 1.3 `newHandler()` и подключение в `main()` вместо `newMux()` — проверка: `go build ./...`
- [ ] 1.4 Табличный тест через `httptest` на `newHandler()`: без заголовка → в ответе валидный UUID; с корректным UUID → он же; с мусором → новый UUID; `405` на `POST /healthz` тоже содержит заголовок — проверка: `go test -race ./...`

## 2. Готовность

- [ ] 2.1 `make check` в `backend/` зелёный
- [ ] 2.2 Архив изменения (`change-archive`): сценарии в `specs/request-id/spec.md`, папка в `changes/archive/`
