# prometheus: изменения сценариев

## ADDED

### Requirement: метрики собираются

#### Scenario: прод

- WHEN Prometheus на проде опрашивает приложение
- THEN цель в состоянии UP, порт Prometheus не открыт снаружи

