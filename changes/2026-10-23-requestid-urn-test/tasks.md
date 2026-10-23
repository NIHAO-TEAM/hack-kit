# Задачи: X-Request-Id в форматах urn и фигурных скобок

## 1. Тест

- [ ] 1.1 Новый файл `backend/cmd/api/requestid_formats_test.go` (не `main_test.go`: параллельные изменения этапа не должны править один файл): Тест: `X-Request-Id: urn:uuid:<uuid>` и `{<uuid>}` → в ответе тот же UUID в каноническом виде (36 символов, нижний регистр). — проверка: `go test -race ./...`

## 2. Готовность

- [ ] 2.1 `make check` в `backend/` зелёный
- [ ] 2.2 Архив изменения (`change-archive`): сценарии в `specs/request-id/spec.md`, папка в `changes/archive/`
