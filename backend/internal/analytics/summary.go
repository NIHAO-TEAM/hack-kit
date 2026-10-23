package analytics

import (
	"example.com/app/internal/platform/httpx"
)

// GetSummary — GET /api/v1/analytics/summary.
func (*AnalyticsAPI) GetSummary(w responseWriter, r *request) { httpx.NotImplemented(w, r) }
