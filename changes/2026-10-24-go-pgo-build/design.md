# Дизайн: PGO и GOMEMLIMIT

## Подход

1. Профили: `go test ./internal/app -run ^$ -bench Recalc100k -cpuprofile recalc.pprof`, то же для чтения списка и приёма потока; слить `go tool pprof -proto a b c > cmd/api/default.pgo`.
2. Сборка: `go build -pgo=auto` (по умолчанию с Go 1.21, если `default.pgo` лежит в пакете `main`); в Dockerfile флаг явно, чтобы было видно.
3. `benchstat` по 10 прогонам до и после.
4. `GOMEMLIMIT`: в Helm — из `resources.limits.memory` (если задан) через переменную окружения; в Compose — константа.

## Допущения

- [ПРЕДПОЛОЖЕНО] Профиль размером до 1 МБ можно коммитить.

## Риски

- Профиль устаревает при правках кода: Go это терпит (сборка не ломается), выигрыш падает. В README — команда обновления.
