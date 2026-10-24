# Задачи: готовность на пустой базе

- [x] 1.1 `Engine.LastError` — ошибка последнего завершённого прогона, отменённые не трогают — проверка: `go test -race ./internal/analytics`
- [x] 1.2 `/readyz` готов при `ErrNoSnapshot`, при другой ошибке — нет — проверка: `TestReadyOnEmptyDatabase`; живой стенд с пустой базой — 200
- [ ] 2.1 `main` после слияния: CI, `Deploy k8s`, smoke, `Deploy chaos` (ручной) зелёные — после слияния
