# ui-polish: изменения сценариев

## ADDED

### Requirement: на «О модели» нет недоступных прокручиваемых областей

#### Scenario: телефон, тёмная тема

- WHEN axe проверяет «О модели» на 390 px в тёмной теме
- THEN нарушений `scrollable-region-focusable` нет
