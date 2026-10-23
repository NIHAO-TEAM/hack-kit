# churn-chart

### Requirement: факт и прогноз различимы

#### Scenario: сводка

- WHEN открыта «Сводка»
- THEN на графике исторические удаления сплошной линией, прогноз — штриховой, граница «сейчас» подписана


Тест: `frontend/src/app/summary.test.tsx`; ряды — `backend/internal/analytics/chart_test.go`.
