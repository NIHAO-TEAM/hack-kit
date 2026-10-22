# Задачи: HEAD /version

## 1. Тест

- [x] 1.1 Тест `HEAD /version`: 200, `Content-Type: application/json`, пустое тело, заголовок `X-Request-Id` — проверка: `go test -race ./...`

## 2. Готовность

- [x] 2.1 `make check` в `backend/` зелёный
- [x] 2.2 Архив изменения (`change-archive`): сценарии в `specs/version/spec.md`, папка в `changes/archive/`
