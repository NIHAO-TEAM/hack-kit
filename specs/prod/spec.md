# prod: публичный прод (Compose + Caddy)

### Requirement: стенд на публичном домене по TLS

Прод SHALL быть доступен по `https://nihao-churn.duckdns.org` с действительным сертификатом.

#### Scenario: открыть стенд

- WHEN браузер открывает `https://nihao-churn.duckdns.org`
- THEN сертификат действителен, открывается панель (или экран входа)

#### Scenario: http

- WHEN клиент запрашивает `http://nihao-churn.duckdns.org/healthz`
- THEN ответ — перенаправление на `https://`

### Requirement: наружу только веб

#### Scenario: порты

- WHEN снаружи проверяются порты хоста
- THEN открыты только 22, 80, 443; база и порт приложения недоступны

### Requirement: выкат из main

#### Scenario: новая версия

- WHEN в `main` слит PR и CI зелёный
- THEN через несколько минут прод отвечает новой версией (`GET /version`)

#### Scenario: сломанная версия

- WHEN новая версия не отвечает на `/healthz`
- THEN прод остаётся на прежней версии, деплой отмечен красным

### Requirement: секреты прода не утекают

#### Scenario: логи деплоя

- WHEN деплой выполнен
- THEN в логах CI, в репозитории и в PR нет паролей учёток и токенов прода
