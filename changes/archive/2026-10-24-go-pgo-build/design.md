# Дизайн: PGO и GOMEMLIMIT

## Подход

1. Профили: `go test ./internal/app -run ^$ -bench Recalc100k -cpuprofile recalc.pprof`, то же для чтения списка и приёма потока; слить `go tool pprof -proto a b c > cmd/api/default.pgo`.
2. Сборка: `go build -pgo=auto` (по умолчанию с Go 1.21, если `default.pgo` лежит в пакете `main`); в Dockerfile флаг явно, чтобы было видно.
3. `benchstat` по 10 прогонам до и после.
4. `GOMEMLIMIT`: в Helm — константа `400MiB` в `env` values (80 % от лимита 512Mi; вычислять из `resources.limits.memory` в шаблоне хрупко из-за единиц, в комментарии оговорено); в Compose — `${GOMEMLIMIT:-1GiB}`.

## Допущения

- [ПРЕДПОЛОЖЕНО] Профиль размером до 1 МБ можно коммитить.

## Риски

- Профиль устаревает при правках кода: Go это терпит (сборка не ломается), выигрыш падает. В README — команда обновления.
