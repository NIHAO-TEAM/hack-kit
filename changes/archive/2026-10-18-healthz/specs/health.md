# health: изменения сценариев

## ADDED

### Requirement: проверка живости

Система SHALL отвечать на `GET /healthz` без авторизации, пока процесс API принимает запросы.

#### Scenario: API жив

- WHEN клиент запрашивает `GET /healthz`
- THEN ответ `200`, `Content-Type: application/json`, тело `{"status":"ok"}`

#### Scenario: неверный метод

- WHEN клиент отправляет `POST /healthz`
- THEN ответ `405`, заголовок `Allow` содержит `GET`
