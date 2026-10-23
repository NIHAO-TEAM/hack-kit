// Package analytics — клиенты, сводка, метрики, график оттока и поток событий. Пока заглушка:
// каждая операция отвечает 501. Наполняют customers-api, summary-metrics, live-updates.
package analytics

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/events"
	"example.com/app/internal/platform/httpx"
)

// Алиасы типов net/http: правило depguard domain-no-io пускает net/http только в http.go,
// service.go и projection.go, а обработчики лежат в файлах по возможностям.
type (
	responseWriter = http.ResponseWriter
	request        = http.Request
)

// AnalyticsAPI обслуживает /customers, /analytics/*, /events, /metrics.
type AnalyticsAPI struct {
	pool *pgxpool.Pool
	bus  *events.Bus
	clk  clock.Clock
	// overview отдаёт последний результат пересчёта; nil, пока сервер не подключил воркер.
	overview OverviewFunc

	regOnce sync.Once
	reg     *prometheus.Registry
}

// New создаёт обработчик модуля; пул, шину и часы создаёт cmd/api.
func New(pool *pgxpool.Pool, bus *events.Bus, clk clock.Clock) *AnalyticsAPI {
	return &AnalyticsAPI{pool: pool, bus: bus, clk: clk}
}

// WithOverview подключает источник последнего результата пересчёта для сводки и метрик.
func (a *AnalyticsAPI) WithOverview(f OverviewFunc) *AnalyticsAPI {
	a.overview = f
	return a
}

// latest — последний результат пересчёта или nil.
func (a *AnalyticsAPI) latest() *Overview {
	if a.overview == nil {
		return nil
	}
	return a.overview()
}

// notReady отвечает 503, пока первый пересчёт не закончился.
func notReady(w responseWriter, r *request) {
	httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "первый пересчёт ещё не закончился")
}

func writeJSON(w responseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
