# Дизайн: GET /healthz с Accept: text/html

## Подход

Табличный тест через `httptest` на `newHandler()` в отдельном файле `healthz_accept_test.go` (пакет `main`). Код не меняется.

## Допущения

- Нет.

## Риски

- Нет.
