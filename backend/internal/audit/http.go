package audit

import (
	"encoding/json"
	"log"
	"net/http"

	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/httpx"
)

func logf(format string, args ...any) { log.Printf("audit: "+format, args...) }

// AuditAPI обслуживает путь /audit. Права проверяет auth.Middleware.
type AuditAPI struct {
	svc *Service
}

// NewAPI создаёт обработчик модуля. svc nil (тесты без базы) — операция отвечает 501.
func NewAPI(svc *Service) *AuditAPI { return &AuditAPI{svc: svc} }

// ListAudit — GET /api/v1/audit.
func (a *AuditAPI) ListAudit(w http.ResponseWriter, r *http.Request, params gen.ListAuditParams) {
	if a.svc == nil || a.svc.pool == nil {
		httpx.NotImplemented(w, r)
		return
	}
	limit, offset := 50, 0
	if params.Limit != nil {
		limit = *params.Limit
	}
	if params.Offset != nil {
		offset = *params.Offset
	}
	if limit < 1 || limit > 200 || offset < 0 {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "limit от 1 до 200, offset не меньше 0")
		return
	}
	items, total, err := a.svc.List(r.Context(), limit, offset)
	if err != nil {
		log.Printf("%v", err)
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "журнал недоступен")
		return
	}
	out := gen.AuditPage{Items: make([]gen.AuditEntry, len(items)), Total: total}
	for i, e := range items {
		changes := make([]gen.AuditChange, len(e.Changes))
		for j, c := range e.Changes {
			changes[j] = gen.AuditChange{Field: c.Field, From: c.From, To: c.To}
		}
		out.Items[i] = gen.AuditEntry{
			Id: e.ID, At: e.At, Actor: e.Actor, Action: gen.AuditEntryAction(e.Action),
			Summary: e.Summary, Changes: changes,
		}
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		log.Printf("audit: ответ: %v", err)
	}
}
