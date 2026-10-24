package analytics

import (
	"context"
	"log"
	"strings"

	"example.com/app/internal/platform/api/gen"
)

// TopicTasks — тема шины: после каждой записи по задаче удержания в неё уходит TaskEvent.
const TopicTasks = "tasks"

// TaskEvent — событие SSE `tasks`: что случилось с задачей и по какому клиенту.
type TaskEvent struct {
	Type       string `json:"type"` // created, changed, event
	TaskID     int64  `json:"taskId"`
	CustomerID int64  `json:"customerId"`
}

// TaskRef — открытая задача клиента. Email — тот, с которым клиента взяли в работу: у другого
// email того же ID задача к строке не привязывается.
type TaskRef struct {
	Email string
	Brief gen.TaskBrief
}

// TaskSource отдаёт открытые задачи по ID клиента; его реализует модуль retention.
type TaskSource interface {
	OpenTasks(ctx context.Context) (map[int64]TaskRef, error)
}

// WithTasks подключает задачи удержания: поле task в строках и фильтр task.
func (a *AnalyticsAPI) WithTasks(src TaskSource, login func(context.Context) string) *AnalyticsAPI {
	a.tasks = src
	a.login = login
	return a
}

// openTasks возвращает открытые задачи; без источника — пусто. Ошибка чтения не прячет очередь:
// вызывающий решает, нужен ли ему результат.
func (a *AnalyticsAPI) openTasks(ctx context.Context) (map[int64]TaskRef, error) {
	if a.tasks == nil {
		return nil, nil
	}
	m, err := a.tasks.OpenTasks(ctx)
	if err != nil {
		log.Printf("analytics: задачи: %v", err)
	}
	return m, err
}

// briefFor возвращает открытую задачу клиента, если она привязана именно к нему.
func briefFor(tasks map[int64]TaskRef, row gen.CustomerRow) *gen.TaskBrief {
	ref, ok := tasks[row.Id]
	if !ok || !strings.EqualFold(strings.TrimSpace(ref.Email), strings.TrimSpace(row.Email)) {
		return nil
	}
	b := ref.Brief
	return &b
}

func validTaskFilter(f gen.ListCustomersParamsTask) bool {
	return f == "none" || f == "open" || f == "mine"
}
