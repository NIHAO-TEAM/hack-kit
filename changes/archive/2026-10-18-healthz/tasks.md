# Задачи: GET /healthz

## 1. Логика

- [x] 1.1 Обработчик `healthz` и регистрация `GET /healthz` в `newMux()` — проверка: `go vet ./...`
- [x] 1.2 Табличный тест через `httptest` на `newMux()`: GET → 200, JSON-тело, `Content-Type`; POST → 405, в `Allow` есть `GET` — проверка: `go test -race ./...`

## 2. Готовность

- [x] 2.1 `make check` в `backend/` зелёный
- [x] 2.2 Архив изменения (`change-archive`): сценарии в `specs/health/spec.md`, папка в `changes/archive/`
