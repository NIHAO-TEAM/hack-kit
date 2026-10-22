# ping

### Requirement: проверка связи

Система SHALL отвечать на `GET /ping` без авторизации и запрещать кэширование ответа.

#### Scenario: API отвечает

- WHEN клиент запрашивает `GET /ping`
- THEN ответ `200`, `Content-Type: application/json`, `Cache-Control: no-store`, тело `{"ping":"pong"}`
- Тест: `TestPing/GET_отвечает_pong`

#### Scenario: неверный метод

- WHEN клиент отправляет `POST /ping`
- THEN ответ `405`, заголовок `Allow` содержит `GET`
- Тест: `TestPing/POST_запрещён`

### Requirement: проверка связи методом HEAD

Система SHALL отвечать на `HEAD /ping` так же, как на `GET`, но без тела.

#### Scenario: HEAD-запрос

- WHEN клиент отправляет `HEAD /ping`
- THEN ответ `200`, тело пустое, `Cache-Control: no-store`, в ответе есть `X-Request-Id`
- Тест: `TestPingHead`
