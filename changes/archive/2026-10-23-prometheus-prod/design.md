# Дизайн: Prometheus на проде: сбор `/metrics`, target UP

## Подход

- Токен сборщика — из `.env` прода (генерирует `bootstrap.sh`), в конфиг Prometheus — через `credentials_file`.
- Prometheus — сервис `prometheus` в `deploy/docker-compose.prod.yml`, образ `prom/prometheus:v2.55.1`; порт только `expose` (наружу не публикуется, Caddy его не проксирует). Конфиг и правила — `deploy/prometheus/`.
- Токен: точка входа пишет `METRICS_TOKEN` из окружения в `/tmp/metrics_token` (umask 077), конфиг ссылается на файл через `credentials_file`.
- Алерты: `ExpectedLossGrowth` (рост > 20 % за час), `SnapshotStale` (`churn_snapshot_age_seconds > 600`), `ChurnTargetDown`.

## Решения по ходу

- [ПРЕДПОЛОЖЕНО] Прод сейчас может работать на k3s (`helm-k3s`): `bootstrap.sh` пропускает Compose при активном k3s. В чарте уже есть `ServiceMonitor` (`serviceMonitor.enabled`, нужен prometheus-operator); Prometheus для k3s в этом изменении не ставлю — только Compose-прод, как сказано в `proposal.md` («Compose прода»). Выбор по порядку важности: меньше работы и риска для параллельных PR.
- [ПРЕДПОЛОЖЕНО] `promtool` и Docker в среде нет: синтаксис YAML проверен локально, `promtool check config/rules` и target UP проверяются при выкате.
