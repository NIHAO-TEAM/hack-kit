package retention

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/httpx"
)

// TasksAPI обслуживает /tasks и POST /customers/{id}/task. Права проверяет auth.Middleware.
type TasksAPI struct {
	svc *Service
}

// NewAPI создаёт обработчик модуля. svc nil или без базы (тесты без базы) — операции отвечают 501.
func NewAPI(svc *Service) *TasksAPI { return &TasksAPI{svc: svc} }

func (a *TasksAPI) ready(w http.ResponseWriter, r *http.Request) bool {
	if a.svc == nil || a.svc.pool == nil {
		httpx.NotImplemented(w, r)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("retention: ответ: %v", err)
	}
}

// decode разбирает тело запроса; пустое тело допустимо, если optional.
func decode(w http.ResponseWriter, r *http.Request, v any, optional bool) bool {
	err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(v)
	if err != nil && (!optional || !errors.Is(err, io.EOF)) {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "тело запроса не разобрано")
		return false
	}
	return true
}

// fail переводит ошибку сервиса в ответ.
func (a *TasksAPI) fail(w http.ResponseWriter, r *http.Request, err error) {
	var conflict *ConflictError
	var invalid *ValidationError
	switch {
	case errors.As(err, &conflict):
		detail := "задачу уже изменил " + conflict.By
		if conflict.Duplicate {
			detail = "клиент уже в работе: задача ведётся у " + conflict.Task.Assignee
		}
		writeConflict(w, r, detail, conflict.Task)
	case errors.As(err, &invalid):
		errs := make([]httpx.FieldError, len(invalid.Fields))
		for i, f := range invalid.Fields {
			errs[i] = httpx.FieldError{Field: f.Field, Message: f.Message}
		}
		httpx.WriteProblem(w, r, http.StatusBadRequest, "запрос не прошёл проверку", errs...)
	case errors.Is(err, ErrNotFound):
		httpx.WriteProblem(w, r, http.StatusNotFound, "задача не найдена")
	case errors.Is(err, ErrCustomerNotFound):
		httpx.WriteProblem(w, r, http.StatusNotFound, "клиент не найден")
	case errors.Is(err, ErrNotReady):
		httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "расчёт ещё не готов")
	default:
		log.Printf("%v", err)
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "задачи недоступны")
	}
}

// writeConflict пишет 409 как problem+json с текущей задачей.
func writeConflict(w http.ResponseWriter, r *http.Request, detail string, task gen.RetentionTask) {
	id := w.Header().Get("X-Request-Id")
	if id == "" {
		id = r.Header.Get("X-Request-Id")
	}
	status := http.StatusConflict
	title := http.StatusText(status)
	body := gen.TaskConflict{Type: "about:blank", Title: title, Status: status, Detail: &detail, CorrelationId: &id, Task: &task}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("retention: ответ: %v", err)
	}
}

// closedConflict — запись в закрытую задачу: 409 с её текущим состоянием.
func (a *TasksAPI) closedConflict(w http.ResponseWriter, r *http.Request, id int64) {
	t, err := a.svc.Get(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeConflict(w, r, "задача закрыта, править её нельзя: возьмите клиента в работу заново", t)
}

// TakeCustomerTask — POST /api/v1/customers/{id}/task.
func (a *TasksAPI) TakeCustomerTask(w http.ResponseWriter, r *http.Request, id gen.CustomerId) {
	if !a.ready(w, r) {
		return
	}
	var body gen.TaskCreate
	if !decode(w, r, &body, true) {
		return
	}
	t, err := a.svc.Take(r.Context(), id, TakeInput{Assignee: body.Assignee, DueAt: body.DueAt, Note: body.Note})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

// ListTasks — GET /api/v1/tasks.
func (a *TasksAPI) ListTasks(w http.ResponseWriter, r *http.Request, p gen.ListTasksParams) {
	if !a.ready(w, r) {
		return
	}
	f := ListFilter{Limit: 50}
	if p.Limit != nil {
		f.Limit = *p.Limit
	}
	if p.Offset != nil {
		f.Offset = *p.Offset
	}
	if f.Limit < 1 || f.Limit > 200 || f.Offset < 0 {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "limit от 1 до 200, offset не меньше 0")
		return
	}
	if p.Status != nil {
		if !validStatus(*p.Status) {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "неизвестный статус")
			return
		}
		f.Status = p.Status
	}
	if p.Assignee != nil {
		f.Assignee = *p.Assignee
		if f.Assignee == "me" {
			f.Assignee = a.svc.login(r.Context())
		}
	}
	f.Overdue = p.Overdue != nil && *p.Overdue
	items, total, err := a.svc.List(r.Context(), f)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, gen.TaskPage{Items: items, Total: total})
}

// ListTaskAssignees — GET /api/v1/tasks/assignees.
func (a *TasksAPI) ListTaskAssignees(w http.ResponseWriter, r *http.Request) {
	if !a.ready(w, r) {
		return
	}
	items, err := a.svc.Assignees(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, gen.TaskAssignees{Items: items})
}

// GetTask — GET /api/v1/tasks/{taskId}.
func (a *TasksAPI) GetTask(w http.ResponseWriter, r *http.Request, id gen.TaskId) {
	if !a.ready(w, r) {
		return
	}
	t, err := a.svc.Get(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// PatchTask — PATCH /api/v1/tasks/{taskId}.
func (a *TasksAPI) PatchTask(w http.ResponseWriter, r *http.Request, id gen.TaskId) {
	if !a.ready(w, r) {
		return
	}
	var body gen.TaskPatch
	if !decode(w, r, &body, false) {
		return
	}
	t, err := a.svc.Patch(r.Context(), id, body)
	if errors.Is(err, ErrClosed) {
		a.closedConflict(w, r, id)
		return
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// AddTaskEvent — POST /api/v1/tasks/{taskId}/events.
func (a *TasksAPI) AddTaskEvent(w http.ResponseWriter, r *http.Request, id gen.TaskId) {
	if !a.ready(w, r) {
		return
	}
	var body gen.TaskEventCreate
	if !decode(w, r, &body, false) {
		return
	}
	t, err := a.svc.AddEvent(r.Context(), id, body)
	if errors.Is(err, ErrClosed) {
		a.closedConflict(w, r, id)
		return
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}
