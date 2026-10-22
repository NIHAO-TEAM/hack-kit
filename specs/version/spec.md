# version

### Requirement: версия сборки

Система SHALL отвечать на `GET /version` без авторизации версией сборки.

#### Scenario: версия не задана

- WHEN переменная `APP_VERSION` не задана или пуста и клиент запрашивает `GET /version`
- THEN ответ `200`, `Content-Type: application/json`, тело `{"version":"dev"}`
- Тест: `TestVersion/версия_не_задана`

#### Scenario: версия задана

- WHEN `APP_VERSION=1.2.3` и клиент запрашивает `GET /version`
- THEN ответ `200`, `Content-Type: application/json`, тело `{"version":"1.2.3"}`
- Тест: `TestVersion/версия_задана`

#### Scenario: неверный метод

- WHEN клиент отправляет `POST /version`
- THEN ответ `405`, заголовок `Allow` содержит `GET`
- Тест: `TestVersion/POST_запрещён`

### Requirement: версия методом HEAD

Система SHALL отвечать на `HEAD /version` так же, как на `GET`, но без тела.

#### Scenario: HEAD-запрос

- WHEN клиент отправляет `HEAD /version`
- THEN ответ `200`, `Content-Type: application/json`, тело пустое, в ответе есть `X-Request-Id`
- Тест: `TestVersionHead`
