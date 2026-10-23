# Задачи: X-Request-Id: регистр имени заголовка

## 1. Тест

- [x] 1.1 Новый файл `backend/cmd/api/requestid_case_test.go` (не `main_test.go`: параллельные изменения этапа не должны править один файл): Тест: запрос с заголовком `x-request-id: <uuid>` (нижний регистр имени) → в ответе `X-Request-Id` равен этому UUID. — проверка: `go test -race ./...`

## 2. Готовность

- [x] 2.1 `make check` в `backend/` зелёный
- [x] 2.2 Архив изменения (`change-archive`): сценарии в `specs/request-id/spec.md`, папка в `changes/archive/`
