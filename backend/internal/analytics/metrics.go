package analytics

import (
	"example.com/app/internal/platform/httpx"
)

// GetMetrics — GET /metrics.
func (*AnalyticsAPI) GetMetrics(w responseWriter, r *request) { httpx.NotImplemented(w, r) }

// GetMetricsAlias — GET /metics, алиас опечатки в кейсе.
func (*AnalyticsAPI) GetMetricsAlias(w responseWriter, r *request) {
	httpx.NotImplemented(w, r)
}
