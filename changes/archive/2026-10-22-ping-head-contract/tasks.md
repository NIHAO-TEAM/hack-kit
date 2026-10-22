# Задачи: HEAD /ping в контракте

## 1. Контракт

- [x] 1.1 В `api/openapi.yaml` у `/ping` операция `head`: параметр `X-Request-Id`, ответ `200` без тела с заголовками `X-Request-Id` и `Cache-Control` — проверка: `[ci] контракт` без ломающих изменений

## 2. Тест

- [x] 2.1 Тест `HEAD /ping`: 200, пустое тело, `Cache-Control: no-store`, `X-Request-Id` — проверка: `go test -race ./...`

## 3. Готовность

- [x] 3.1 `make check` в `backend/` зелёный
- [x] 3.2 Архив изменения (`change-archive`): сценарии в `specs/ping/spec.md`, папка в `changes/archive/`
