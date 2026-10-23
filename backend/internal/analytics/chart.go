package analytics

import (
	"net/http"

	"example.com/app/internal/platform/httpx"
)

// GetChurnChart — GET /api/v1/analytics/churn-chart (слой 2).
func (*AnalyticsAPI) GetChurnChart(w http.ResponseWriter, r *http.Request) {
	httpx.NotImplemented(w, r)
}
