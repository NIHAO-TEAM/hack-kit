# Задачи: GET /version

## 1. Логика

- [x] 1.1 Обработчик `version` и регистрация `GET /version` в `newMux()` — проверка: `go vet ./...`
- [x] 1.2 Табличный тест через `httptest` на `newMux()`: GET без `APP_VERSION` → `{"version":"dev"}`; GET с `APP_VERSION=1.2.3` → `{"version":"1.2.3"}`; проверить статус и `Content-Type`; POST → 405 с `GET` в `Allow` — проверка: `go test -race ./...`

## 2. Готовность

- [x] 2.1 `make check` в `backend/` зелёный
- [x] 2.2 Архив изменения (`change-archive`): сценарии в `specs/version/spec.md`, папка в `changes/archive/`
