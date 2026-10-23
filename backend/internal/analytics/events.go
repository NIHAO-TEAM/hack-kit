package analytics

import (
	"example.com/app/internal/platform/httpx"
)

// GetEvents — GET /api/v1/events (SSE).
func (*AnalyticsAPI) GetEvents(w responseWriter, r *request) { httpx.NotImplemented(w, r) }
