# Задачи: Compose и README

Каждая задача проверяема: в конце строки — чем проверить.

## 1. Контракт и данные

- нет

## 2. Логика

- [x] 2.1 `internal/platform/webui`: `embed` + откат на `index.html` для SPA — проверка: unit-тест «`/queue` → `index.html`; `/api/v1/x` не перехватывается; `/assets/a.js` отдаётся»
- [ ] 2.2 `Dockerfile` (три ступени, distroless nonroot, условная сборка фронта) — проверка: `docker build .` проходит без фронта; `docker run --rm <образ> id` недоступен (нет shell) и образ запускается от `nonroot`
- [ ] 2.3 `docker-compose.yml` (postgres без порта, healthcheck, app ждёт базу) — проверка: `docker compose up -d && curl -fsS localhost:8080/healthz`
- [ ] 2.4 `.env.example` без секретов, `.env` в `.gitignore`, понятная ошибка при пустом `ADMIN_PASSWORD` — проверка: `git check-ignore .env`; старт без пароля → лог с именем переменной

## 3. Экран

- нет

## 4. Готовность

- [ ] 4.1 `make check` зелёный
- [ ] 4.2 Сценарии из `specs/` проходят: с чистой копии репозитория по README за 3 команды открывается `/api/v1/docs`
- [ ] 4.3 Корневой `README.md` — README продукта; описание кита — в `hack-kit/` без потерь
