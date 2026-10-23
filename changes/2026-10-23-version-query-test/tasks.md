# Задачи: GET /version с параметрами запроса

## 1. Тест

- [ ] 1.1 Новый файл `backend/cmd/api/version_query_test.go` (не `main_test.go`: параллельные изменения этапа не должны править один файл): Тест: `GET /version?x=1` → `200`, в теле поле `version`. — проверка: `go test -race ./...`

## 2. Готовность

- [ ] 2.1 `make check` в `backend/` зелёный
- [ ] 2.2 Архив изменения (`change-archive`): сценарии в `specs/version/spec.md`, папка в `changes/archive/`
