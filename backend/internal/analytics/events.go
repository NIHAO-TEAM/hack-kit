package analytics

import (
	"net/http"

	"example.com/app/internal/platform/httpx"
)

// GetEvents — GET /api/v1/events (SSE).
func (*AnalyticsAPI) GetEvents(w http.ResponseWriter, r *http.Request) { httpx.NotImplemented(w, r) }
