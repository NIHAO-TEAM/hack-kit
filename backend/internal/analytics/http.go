// Package analytics — клиенты, сводка, метрики, график оттока и поток событий. Пока заглушка:
// каждая операция отвечает 501. Наполняют customers-api, summary-metrics, live-updates.
package analytics

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/events"
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
