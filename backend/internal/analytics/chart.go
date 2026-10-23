package analytics

import (
	"example.com/app/internal/platform/httpx"
)

// GetChurnChart — GET /api/v1/analytics/churn-chart (слой 2).
func (*AnalyticsAPI) GetChurnChart(w responseWriter, r *request) {
	httpx.NotImplemented(w, r)
}
