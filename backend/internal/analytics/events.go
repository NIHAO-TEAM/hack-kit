package analytics

import (
	"encoding/json"
	"fmt"
	"time"
)

// heartbeatEvery — как часто в поток уходит пульс-комментарий, чтобы прокси не рвал простаивающее соединение.
const heartbeatEvery = 15 * time.Second

// GetEvents — GET /api/v1/events (SSE): после каждого пересчёта событие recalculated с номером версии.
func (a *AnalyticsAPI) GetEvents(w responseWriter, r *request) {
	events, cancel := a.bus.Subscribe(TopicRecalculated)
	defer cancel()
	taskEvents, cancelTasks := a.bus.Subscribe(TopicTasks)
	defer cancelTasks()

	// Поток живёт дольше ReadTimeout/IdleTimeout сервера: дедлайны с него снимаются.
	clearDeadlines(w)

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")
	_, _ = fmt.Fprint(w, ": connected\n\n")
	if flush(w) != nil {
		return
	}

	tick := time.NewTicker(heartbeatEvery)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			// Клиент ушёл или сервер останавливается: просим переподключиться (новому поду).
			_, _ = fmt.Fprint(w, "retry: 1000\n\n")
			_ = flush(w)
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			_, _ = fmt.Fprintf(w, "event: recalculated\ndata: {\"type\":\"recalculated\",\"version\":%v}\n\n", ev.Payload)
		case ev, ok := <-taskEvents:
			if !ok {
				return
			}
			te, _ := ev.Payload.(TaskEvent)
			data, _ := json.Marshal(te)
			_, _ = fmt.Fprintf(w, "event: tasks\ndata: %s\n\n", data)
		case <-tick.C:
			_, _ = fmt.Fprint(w, ": ping\n\n")
		}
		if flush(w) != nil {
			return
		}
	}
}
