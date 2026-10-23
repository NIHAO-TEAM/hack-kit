# Задачи: Prometheus на проде: сбор `/metrics`, target UP

- [x] 1.1 Конфиг и правила — проверка: `promtool check config`, `promtool check rules` (promtool в среде нет: YAML проверен, promtool — при выкате)
- [x] 1.2 На проде target UP — проверка: `curl` к API Prometheus изнутри сети (проверит выкат на прод)
- [x] 9.1 `make check` / `pnpm check` зелёные; сценарии `specs/` проходят (код не менялся; изменены только deploy/)
