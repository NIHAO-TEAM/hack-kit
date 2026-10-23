# Дизайн: Prometheus на проде: сбор `/metrics`, target UP

## Подход

- Токен сборщика — из `.env` прода (генерирует `bootstrap.sh`), в конфиг Prometheus — через `credentials_file`.
