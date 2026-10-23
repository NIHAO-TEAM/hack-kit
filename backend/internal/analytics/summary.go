package analytics

import (
	"net/http"

	"example.com/app/internal/platform/httpx"
)

// GetSummary — GET /api/v1/analytics/summary.
func (*AnalyticsAPI) GetSummary(w http.ResponseWriter, r *http.Request) { httpx.NotImplemented(w, r) }
