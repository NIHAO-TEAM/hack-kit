# prod

### Requirement: прод проверяется после каждого выката

Система SHALL после каждого успешного выката проверять прод в браузере и сообщать о сбое.

#### Scenario: выкат прошёл

- WHEN `deploy.yml` или `deploy-k8s.yml` завершился успехом
- THEN `deploy-smoke.yml` открывает прод в Firefox: главная, Swagger, 401 без входа, `/metrics` закрыт; при пароле `jury` — очередь и карточка
- Тест: `e2e/tests/smoke.spec.ts`

#### Scenario: прод сломан

- WHEN дымовая проверка падает
- THEN запуск красный и открыта одна issue `prod-down` со ссылкой на запуск
