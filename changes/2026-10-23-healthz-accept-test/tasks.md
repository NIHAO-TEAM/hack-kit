# Задачи: GET /healthz с Accept: text/html

## 1. Тест

- [x] 1.1 Новый файл `backend/cmd/api/healthz_accept_test.go` (не `main_test.go`: параллельные изменения этапа не должны править один файл): Тест: `GET /healthz` с `Accept: text/html` → `200`, `Content-Type: application/json`, тело `{"status":"ok"}`. — проверка: `go test -race ./...`

## 2. Готовность

- [ ] 2.1 `make check` в `backend/` зелёный
- [ ] 2.2 Архив изменения (`change-archive`): сценарии в `specs/health/spec.md`, папка в `changes/archive/`
