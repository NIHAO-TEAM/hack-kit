# Задачи: HEAD /healthz

## 1. Тест

- [x] 1.1 Тест `HEAD /healthz`: 200, `Content-Type: application/json`, пустое тело, заголовок `X-Request-Id` — проверка: `go test -race ./...`

## 2. Готовность

- [ ] 2.1 `make check` в `backend/` зелёный
- [ ] 2.2 Архив изменения (`change-archive`): сценарии в `specs/health/spec.md`, папка в `changes/archive/`
