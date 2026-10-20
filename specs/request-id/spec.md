# request-id

### Requirement: идентификатор запроса

Система SHALL возвращать в каждом ответе заголовок `X-Request-Id` с UUID.

#### Scenario: заголовка в запросе нет

- WHEN клиент запрашивает `GET /healthz` без `X-Request-Id`
- THEN в ответе есть `X-Request-Id` со значением, разбираемым как UUID
- Тест: `TestRequestID/заголовка_нет`

#### Scenario: клиент прислал UUID

- WHEN клиент запрашивает `GET /healthz` с `X-Request-Id`, равным корректному UUID
- THEN в ответе `X-Request-Id` равен присланному UUID в канонической форме (нижний регистр, с дефисами)
- Тест: `TestRequestID/корректный_UUID_возвращается`

#### Scenario: UUID в верхнем регистре

- WHEN клиент запрашивает `GET /healthz` с `X-Request-Id` в верхнем регистре
- THEN в ответе `X-Request-Id` — тот же UUID в нижнем регистре
- Тест: `TestRequestID/верхний_регистр_нормализуется`

#### Scenario: клиент прислал мусор

- WHEN клиент запрашивает `GET /healthz` с `X-Request-Id: not-a-uuid`
- THEN в ответе `X-Request-Id` — новый UUID, не равный присланной строке
- Тест: `TestRequestID/мусор_заменяется`

#### Scenario: ответ с ошибкой

- WHEN клиент отправляет `POST /healthz`
- THEN ответ `405` и в нём есть `X-Request-Id`
- Тест: `TestRequestID/405_тоже_с_заголовком`
