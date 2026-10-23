// Package source — снимок исходных данных, правки и эмулятор биллинга. Пока заглушка:
// каждая операция отвечает 501. Наполняет изменение snapshot-ingest.
package source

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/events"
	"example.com/app/internal/platform/httpx"
)

// SourceAPI обслуживает пути /source/*.
type SourceAPI struct {
	pool     *pgxpool.Pool
	bus      *events.Bus
	clk      clock.Clock
	maxBytes int64
}

// defaultMaxBytes — лимит тела снимка по умолчанию (SNAPSHOT_MAX_BYTES), 512 МБ.
const defaultMaxBytes = 512 << 20

// New создаёт обработчик модуля; пул, шину и часы создаёт cmd/api.
func New(pool *pgxpool.Pool, bus *events.Bus, clk clock.Clock) *SourceAPI {
	max := int64(defaultMaxBytes)
	if v, err := strconv.ParseInt(os.Getenv("SNAPSHOT_MAX_BYTES"), 10, 64); err == nil && v > 0 {
		max = v
	}
	return &SourceAPI{pool: pool, bus: bus, clk: clk, maxBytes: max}
}

// PutSnapshot — PUT /api/v1/source/snapshot. Права проверяет auth.Middleware.
func (a *SourceAPI) PutSnapshot(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, a.maxBytes)
	s, errs, err := ParseSnapshot(r.Body)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "тело не разобрано: "+err.Error())
		return
	}
	errs = append(errs, Validate(s)...)
	if len(errs) > 0 {
		if len(errs) > maxErrors {
			errs = errs[:maxErrors]
		}
		fe := make([]httpx.FieldError, len(errs))
		for i, e := range errs {
			fe[i] = httpx.FieldError{Field: e.Field, Message: e.Message}
		}
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "снимок не прошёл проверку", fe...)
		return
	}
	now := a.clk.Now()
	acc, err := replace(r.Context(), a.pool, s, ModelTime(s, now), now, OriginAPI)
	if err != nil {
		log.Printf("source: запись снимка: %v", err)
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "снимок не записан, прежние данные сохранены")
		return
	}
	a.bus.Publish(TopicChanged, acc.Version)
	writeJSON(w, gen.SnapshotAccepted{
		Version: acc.Version, AsOf: acc.AsOf,
		Accounts: acc.Accounts, Services: acc.Services, Transactions: acc.Transactions,
	})
}

// GetSourceStatus — GET /api/v1/source/status. Без снимков: версия 0.
func (a *SourceAPI) GetSourceStatus(w http.ResponseWriter, r *http.Request) {
	st, ok, err := currentStatus(r.Context(), a.pool)
	if err != nil {
		log.Printf("source: статус: %v", err)
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "статус недоступен")
		return
	}
	now := a.clk.Now()
	out := gen.SourceStatus{LoadedAt: now, Checks: []gen.QualityCheck{}}
	if ok {
		src := gen.SourceStatusSource(st.Source)
		out.Version, out.Source, out.AsOf, out.LoadedAt = st.Version, &src, st.AsOf, st.LoadedAt
		out.AgeSeconds = int64(now.Sub(st.LoadedAt).Seconds())
	}
	writeJSON(w, out)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("source: ответ: %v", err)
	}
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
