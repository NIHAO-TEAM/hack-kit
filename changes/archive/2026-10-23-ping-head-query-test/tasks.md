# Задачи: HEAD /ping с параметрами запроса

## 1. Тест

- [x] 1.1 Новый файл `backend/cmd/api/ping_head_query_test.go` (не `main_test.go`: параллельные изменения этапа не должны править один файл): Тест: `HEAD /ping?x=1` → `200`, пустое тело, `Cache-Control: no-store`. — проверка: `go test -race ./...`

## 2. Готовность

- [x] 2.1 `make check` в `backend/` зелёный
- [x] 2.2 Архив изменения (`change-archive`): сценарии в `specs/ping/spec.md`, папка в `changes/archive/`
