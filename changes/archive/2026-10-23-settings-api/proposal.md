Статус: на ревью

# `GET/PATCH /model/settings` с предпросмотром `dryRun`

## Зачем

Кейс: «Коэффициенты isk-score редактируются через панель. Изменения влияют на расчёт и сохраняются после перезапуска»; приёмка: «Изменение коэффициента через панель вызывает пересчёт без reload» (`docs/case.md`). Защита, шаг 3.

## Что меняется

- `GET /api/v1/model/settings` — веса, пороги, горизонт, `priceBasis`, версия, кто и когда менял.
- `PATCH /api/v1/model/settings` — частичная правка; вес вне [0, 1], `high ≤ medium`, горизонт вне 1..365, неизвестный признак — `422` с ошибками по полям; успех сохраняет новую версию и запускает пересчёт.
- `PATCH …?dryRun=true` — ничего не сохраняет, отвечает «сменят уровень N клиентов, потеря ±X €» (`DryRunResult`).

Вне изменения:
- Экран модели — `model-screen`. Права уже проверяет `auth-sessions`.

## Затронутые части

- backend: `internal/model/{http,dryrun,settings,service}.go`, `internal/app/settings.go`, `internal/risk/settings.go` (`Problems`), `internal/auth/http.go` (`LoginFrom`)
- frontend: нет
- контракт `api/openapi.yaml`: добавления — ответ `422` у `PATCH /model/settings`, необязательные `updatedBy`, `updatedAt` в `ModelSettings` (сгенерированные `backend/internal/platform/api/gen`, `docs/openapi.yaml`, `frontend/src/shared/api/schema.gen.ts`)
- защищённые пути: `api/openapi.yaml` — добавление `422` и двух необязательных полей, без которых нельзя выполнить «422 по полям» и «кто и когда менял»
- зависит от: `recalc-engine`

## Связанные решения

- ADR: 0001
- Макет: не нужен

## Найдено по ходу

- `TestGolden` в `internal/risk` падает на `main` (`bonus_locked` в эталоне `testdata/golden/stand.json` отсутствует) — не из этого изменения, не трогал.

