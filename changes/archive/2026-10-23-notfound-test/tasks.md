# Задачи: 404 на неизвестный путь

## 1. Тест

- [x] 1.1 Новый файл `backend/cmd/api/notfound_test.go` (не `main_test.go`: параллельные изменения этапа не должны править один файл): Тест: `GET /nope` и `GET /healthz/extra` → `404`, в ответе есть `X-Request-Id`. — проверка: `go test -race ./...`

## 2. Готовность

- [x] 2.1 `make check` в `backend/` зелёный
- [x] 2.2 Архив изменения (`change-archive`): сценарии в `specs/routing/spec.md`, папка в `changes/archive/`
