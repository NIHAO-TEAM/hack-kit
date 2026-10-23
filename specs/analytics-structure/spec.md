# analytics-structure

### Requirement: поведение не меняется

#### Scenario: пути аналитики

- WHEN клиент вызывает любой путь `analytics` до и после изменения
- THEN ответы одинаковы
- Тест: `internal/analytics` (`go test`), обработчики-заглушки отвечают 501
