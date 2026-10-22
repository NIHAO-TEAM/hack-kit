# Дизайн: HEAD /version

## Подход

Тест через `httptest` на `newHandler()`: `HEAD /version` → `200`, `Content-Type: application/json`, тело пустое, есть `X-Request-Id`. Код не меняется.

## Допущения

- Нет.

## Риски

- Нет.
