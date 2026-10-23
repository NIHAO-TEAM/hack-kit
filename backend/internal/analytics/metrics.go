package analytics

import (
	"net/http"

	"example.com/app/internal/platform/httpx"
)

// GetMetrics — GET /metrics.
func (*AnalyticsAPI) GetMetrics(w http.ResponseWriter, r *http.Request) { httpx.NotImplemented(w, r) }

// GetMetricsAlias — GET /metics, алиас опечатки в кейсе.
func (*AnalyticsAPI) GetMetricsAlias(w http.ResponseWriter, r *http.Request) {
	httpx.NotImplemented(w, r)
}
