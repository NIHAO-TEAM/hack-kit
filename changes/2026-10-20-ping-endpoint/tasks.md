# Задачи: GET /ping

## 1. Логика

- [x] 1.1 Обработчик `ping` и регистрация `GET /ping` в `newMux()` — проверка: `go vet ./...`
- [x] 1.2 Табличный тест через `httptest` на `newHandler()`: GET → 200, `Content-Type: application/json`, `Cache-Control: no-store`, тело `{"ping":"pong"}`; POST → 405, в `Allow` есть `GET` — проверка: `go test -race ./...`

## 2. Готовность

- [ ] 2.1 `make check` в `backend/` зелёный
- [ ] 2.2 Архив изменения (`change-archive`): сценарии в `specs/ping/spec.md`, папка в `changes/archive/`
