# Дизайн: коды ошибок в контракте

## Подход

1. По коду найти фактические ответы: `auth/http.go` (429), `source/http.go` (`MaxBytesReader` → какой код пишет обработчик), middleware CSRF (код и текст).
2. Добавить в `openapi.yaml` общие ответы `TooManyRequests`, `PayloadTooLarge` (если 413), `CsrfRequired` (403) в `components/responses` и сослаться на них из путей.
3. Тест: запросы, вызывающие эти ответы, проверяются валидатором ответа по спецификации (если в проекте уже есть валидатор — `kin-openapi` `openapi3filter`, иначе сверка кода и `Content-Type: application/problem+json`).

## Допущения

- [ПРЕДПОЛОЖЕНО] 403 CSRF документируется у всех путей с `cookieAuth`, которые меняют данные (POST, PUT, PATCH), включая `logout`.

## Риски

- Параллельно `openapi.yaml` правят `billing-queue-split`, `retention-tasks-api`: при конфликте — перенести свои ответы поверх `main`, сгенерированные файлы перегенерировать.
