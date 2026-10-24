package source

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/httpx"
)

// eventsIntake создаёт приём при первом запросе: писатель стартует один раз за жизнь процесса.
func (a *SourceAPI) eventsIntake() *eventIntake {
	a.intakeOnce.Do(func() {
		if a.intake == nil {
			a.intake = newEventIntake(a.pool, a.clk.Now, func(version int64) { a.bus.Publish(TopicChanged, version) })
		}
	})
	return a.intake
}

// PostSourceEvents — POST /api/v1/source/events. Права проверяет auth.Middleware.
// Тело — NDJSON или JSON-массив; события идут тому же писателю пачек, что у эмулятора. Применение
// асинхронное: ответ 202 говорит, что события приняты в очередь, а не что они уже записаны.
func (a *SourceAPI) PostSourceEvents(w http.ResponseWriter, r *http.Request) {
	if a.pool == nil && a.intake == nil {
		httpx.WriteProblem(w, r, http.StatusNotImplemented, "база не подключена: приём событий недоступен")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, eventsMaxBytes)
	out, err := a.eventsIntake().process(r.Context(), r.Body)
	var tooBig *http.MaxBytesError
	var bodyErr eventsBodyError
	switch {
	case errors.As(err, &tooBig):
		httpx.WriteProblem(w, r, http.StatusRequestEntityTooLarge, "тело больше 2 МБ")
		return
	case errors.Is(err, errEventsTooMany):
		httpx.WriteProblem(w, r, http.StatusRequestEntityTooLarge, err.Error())
		return
	case errors.As(err, &bodyErr):
		httpx.WriteProblem(w, r, http.StatusBadRequest, "тело не разобрано: "+bodyErr.Error())
		return
	case errors.Is(err, errNoSnapshot):
		httpx.WriteProblem(w, r, http.StatusBadRequest, "снимка данных ещё нет: сначала загрузите снимок (PUT /source/snapshot)")
		return
	case errors.Is(err, errEventsBusy):
		w.Header().Set("Retry-After", strconv.Itoa(1))
		httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "очередь записи событий полна, ничего не принято: повторите запрос")
		return
	case err != nil:
		log.Printf("source: приём событий: %v", err)
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "события не приняты")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	writeJSONBody(w, gen.IngestAccepted{
		Accepted: out.accepted, Duplicate: out.duplicate, Rejected: out.rejected, Version: out.version,
	})
}
