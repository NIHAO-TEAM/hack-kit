// Package model — настройки и коэффициенты модели. Пока заглушка: каждая операция
// отвечает 501. Наполняет изменение settings-api.
package model

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/events"
	"example.com/app/internal/platform/httpx"
)

// ModelAPI обслуживает пути /model/*.
type ModelAPI struct {
	pool *pgxpool.Pool
	bus  *events.Bus
	clk  clock.Clock
}

// New создаёт обработчик модуля; пул, шину и часы создаёт cmd/api.
func New(pool *pgxpool.Pool, bus *events.Bus, clk clock.Clock) *ModelAPI {
	return &ModelAPI{pool: pool, bus: bus, clk: clk}
}

// GetModelSettings — GET /api/v1/model/settings.
func (*ModelAPI) GetModelSettings(w http.ResponseWriter, r *http.Request) {
	httpx.NotImplemented(w, r)
}

// PatchModelSettings — PATCH /api/v1/model/settings.
func (*ModelAPI) PatchModelSettings(w http.ResponseWriter, r *http.Request, _ gen.PatchModelSettingsParams) {
	httpx.NotImplemented(w, r)
}
