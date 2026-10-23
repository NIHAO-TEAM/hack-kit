// Package analytics — клиенты, сводка, метрики, график оттока и поток событий. Пока заглушка:
// каждая операция отвечает 501. Наполняют customers-api, summary-metrics, live-updates.
package analytics

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/events"
	"example.com/app/internal/platform/httpx"
)

// AnalyticsAPI обслуживает /customers, /analytics/*, /events, /metrics.
type AnalyticsAPI struct {
	pool *pgxpool.Pool
	bus  *events.Bus
	clk  clock.Clock
}

// New создаёт обработчик модуля; пул, шину и часы создаёт cmd/api.
func New(pool *pgxpool.Pool, bus *events.Bus, clk clock.Clock) *AnalyticsAPI {
	return &AnalyticsAPI{pool: pool, bus: bus, clk: clk}
}

// ListCustomers — GET /api/v1/customers.
func (*AnalyticsAPI) ListCustomers(w http.ResponseWriter, r *http.Request, _ gen.ListCustomersParams) {
	httpx.NotImplemented(w, r)
}

// GetCustomer — GET /api/v1/customers/{id}.
func (*AnalyticsAPI) GetCustomer(w http.ResponseWriter, r *http.Request, _ gen.CustomerId) {
	httpx.NotImplemented(w, r)
}

// GetSummary — GET /api/v1/analytics/summary.
func (*AnalyticsAPI) GetSummary(w http.ResponseWriter, r *http.Request) { httpx.NotImplemented(w, r) }

// GetChurnChart — GET /api/v1/analytics/churn-chart (слой 2).
func (*AnalyticsAPI) GetChurnChart(w http.ResponseWriter, r *http.Request) {
	httpx.NotImplemented(w, r)
}

// GetEvents — GET /api/v1/events (SSE).
func (*AnalyticsAPI) GetEvents(w http.ResponseWriter, r *http.Request) { httpx.NotImplemented(w, r) }

// GetMetrics — GET /metrics.
func (*AnalyticsAPI) GetMetrics(w http.ResponseWriter, r *http.Request) { httpx.NotImplemented(w, r) }

// GetMetricsAlias — GET /metics, алиас опечатки в кейсе.
func (*AnalyticsAPI) GetMetricsAlias(w http.ResponseWriter, r *http.Request) {
	httpx.NotImplemented(w, r)
}
