// Package source — снимок исходных данных, правки и эмулятор биллинга. Пока заглушка:
// каждая операция отвечает 501. Наполняет изменение snapshot-ingest.
package source

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/events"
	"example.com/app/internal/platform/httpx"
)

// SourceAPI обслуживает пути /source/*.
type SourceAPI struct {
	pool *pgxpool.Pool
	bus  *events.Bus
	clk  clock.Clock
}

// New создаёт обработчик модуля; пул, шину и часы создаёт cmd/api.
func New(pool *pgxpool.Pool, bus *events.Bus, clk clock.Clock) *SourceAPI {
	return &SourceAPI{pool: pool, bus: bus, clk: clk}
}

// OnStart загружает сид при пустой базе. Заглушка ничего не делает.
func OnStart(context.Context) error { return nil }

// PutSnapshot — PUT /api/v1/source/snapshot.
func (*SourceAPI) PutSnapshot(w http.ResponseWriter, r *http.Request) { httpx.NotImplemented(w, r) }

// GetSourceStatus — GET /api/v1/source/status.
func (*SourceAPI) GetSourceStatus(w http.ResponseWriter, r *http.Request) {
	httpx.NotImplemented(w, r)
}

// PatchService — PATCH /api/v1/source/services/{id}.
func (*SourceAPI) PatchService(w http.ResponseWriter, r *http.Request, _ gen.ServiceId) {
	httpx.NotImplemented(w, r)
}

// PatchAccount — PATCH /api/v1/source/accounts/{id}.
func (*SourceAPI) PatchAccount(w http.ResponseWriter, r *http.Request, _ gen.AccountId) {
	httpx.NotImplemented(w, r)
}

// EmulatorTick — POST /api/v1/source/emulator/tick.
func (*SourceAPI) EmulatorTick(w http.ResponseWriter, r *http.Request) { httpx.NotImplemented(w, r) }

// EmulatorActions — POST /api/v1/source/emulator/actions.
func (*SourceAPI) EmulatorActions(w http.ResponseWriter, r *http.Request) {
	httpx.NotImplemented(w, r)
}
