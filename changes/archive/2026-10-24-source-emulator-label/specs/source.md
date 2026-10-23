# source: изменения сценариев

## ADDED

### Requirement: происхождение снимка «эмулятор»

Система SHALL помечать снимок, записанный эмулятором, происхождением `emulator`.

#### Scenario: жюри включает эмулятор

- WHEN выполнен `POST /api/v1/source/emulator/tick`
- THEN `GET /api/v1/source/status` отдаёт `source: emulator`
- AND экран «Источник» показывает в шапке «Эмулятор»
