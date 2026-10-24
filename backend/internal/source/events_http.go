package source

import (
	"net/http"

	"example.com/app/internal/platform/httpx"
)

// PostSourceEvents — POST /api/v1/source/events. Права проверяет auth.Middleware.
func (a *SourceAPI) PostSourceEvents(w http.ResponseWriter, r *http.Request) {
	httpx.WriteProblem(w, r, http.StatusNotImplemented, "приём событий ещё не реализован")
}
