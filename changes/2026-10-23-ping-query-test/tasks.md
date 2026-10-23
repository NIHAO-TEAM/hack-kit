# Задачи: GET /ping с параметрами запроса

## 1. Тест

- [ ] 1.1 Новый файл `backend/cmd/api/ping_query_test.go` (не `main_test.go`: параллельные изменения этапа не должны править один файл): Тест: `GET /ping?x=1` и `GET /ping?t=1700000000` → `200`, тело `{"ping":"pong"}`, `Cache-Control: no-store`. — проверка: `go test -race ./...`

## 2. Готовность

- [ ] 2.1 `make check` в `backend/` зелёный
- [ ] 2.2 Архив изменения (`change-archive`): сценарии в `specs/ping/spec.md`, папка в `changes/archive/`
