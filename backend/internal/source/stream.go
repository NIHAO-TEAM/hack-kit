package source

import (
	"net/http"

	"example.com/app/internal/platform/httpx"
)

// StartEmulatorStream — POST /api/v1/source/emulator/stream. Наполняет задача 1.2.
func (a *SourceAPI) StartEmulatorStream(w http.ResponseWriter, r *http.Request) {
	httpx.WriteProblem(w, r, http.StatusNotImplemented, "поток событий ещё не реализован")
}

// GetEmulatorStream — GET /api/v1/source/emulator/stream. Наполняет задача 1.2.
func (a *SourceAPI) GetEmulatorStream(w http.ResponseWriter, r *http.Request) {
	httpx.WriteProblem(w, r, http.StatusNotImplemented, "поток событий ещё не реализован")
}
