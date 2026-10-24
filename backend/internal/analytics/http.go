// Package analytics — клиенты, сводка, метрики, график оттока и поток событий. Пока заглушка:
// каждая операция отвечает 501. Наполняют customers-api, summary-metrics, live-updates.
package analytics

import (
	"context"
	"encoding/json"
	"log"
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
	feed Feed
	// tasks и login — задачи удержания для поля task и фильтра; nil, пока сервер не подключил модуль.
	tasks TaskSource
	login func(context.Context) string
	// overview отдаёт последний результат пересчёта; nil, пока сервер не подключил воркер.
	overview OverviewFunc

	extra   []prometheus.Collector
	regOnce sync.Once
	reg     *prometheus.Registry
}

// New создаёт обработчик модуля; пул, шину и часы создаёт cmd/api.
func New(pool *pgxpool.Pool, bus *events.Bus, clk clock.Clock) *AnalyticsAPI {
	return &AnalyticsAPI{pool: pool, bus: bus, clk: clk}
}

// WithFeed подключает источник портфеля клиентов; без него клиенты отвечают 503.
func (a *AnalyticsAPI) WithFeed(f Feed) *AnalyticsAPI {
	a.feed = f
	return a
}

// WithCollectors добавляет в /metrics метрики других модулей; вызывать до первого запроса.
func (a *AnalyticsAPI) WithCollectors(c ...prometheus.Collector) *AnalyticsAPI {
	a.extra = append(a.extra, c...)
	return a
}

func (a *AnalyticsAPI) portfolio() *Portfolio {
	if a.feed == nil {
		return nil
	}
	return a.feed.Portfolio()
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

func writeJSON(w http.ResponseWriter, _ *http.Request, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("analytics: ответ: %v", err)
	}
}

// flush сбрасывает буфер ответа клиенту; работает и через обёртки, у которых есть Unwrap.
func flush(w responseWriter) error { return http.NewResponseController(w).Flush() }
