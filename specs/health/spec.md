# health

### Requirement: проверка живости

Система SHALL отвечать на `GET /healthz` без авторизации, пока процесс API принимает запросы.

#### Scenario: API жив

- WHEN клиент запрашивает `GET /healthz`
- THEN ответ `200`, `Content-Type: application/json`, тело `{"status":"ok"}`
- Тест: `TestHealthz/GET_отвечает_ok`

#### Scenario: неверный метод

- WHEN клиент отправляет `POST /healthz`
- THEN ответ `405`, заголовок `Allow` содержит `GET`
- Тест: `TestHealthz/POST_запрещён`

### Requirement: проверка живости методом HEAD

Система SHALL отвечать на `HEAD /healthz` так же, как на `GET`, но без тела.

#### Scenario: HEAD-запрос

- WHEN клиент отправляет `HEAD /healthz`
- THEN ответ `200`, `Content-Type: application/json`, тело пустое, в ответе есть `X-Request-Id`
- Тест: `TestHealthzHead`
