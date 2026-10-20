# ping: изменения сценариев

## ADDED

### Requirement: проверка связи

Система SHALL отвечать на `GET /ping` без авторизации и запрещать кэширование ответа.

#### Scenario: API отвечает

- WHEN клиент запрашивает `GET /ping`
- THEN ответ `200`, `Content-Type: application/json`, `Cache-Control: no-store`, тело `{"ping":"pong"}`

#### Scenario: неверный метод

- WHEN клиент отправляет `POST /ping`
- THEN ответ `405`, заголовок `Allow` содержит `GET`
