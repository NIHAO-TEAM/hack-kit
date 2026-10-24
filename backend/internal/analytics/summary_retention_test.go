package analytics_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"example.com/app/internal/analytics"
	"example.com/app/internal/platform/api/gen"
)

type fakeTasks struct {
	open map[int64]analytics.TaskRef
	err  error
}

func (f fakeTasks) OpenTasks(context.Context) (map[int64]analytics.TaskRef, error) {
	return f.open, f.err
}

type fakeFeed struct{ pf *analytics.Portfolio }

func (f fakeFeed) Portfolio() *analytics.Portfolio { return f.pf }

func row(id int64, level gen.RiskLevel, loss int64) analytics.Customer {
	email := "c" + string(rune('0'+id%10)) + "@example.test"
	return analytics.Customer{Row: gen.CustomerRow{Id: id, Email: email, Level: level, ExpectedLossCents: loss}}
}

func ref(id int64, status gen.TaskStatus, overdue bool) analytics.TaskRef {
	return analytics.TaskRef{
		Email: "c" + string(rune('0'+id%10)) + "@example.test",
		Brief: gen.TaskBrief{Id: id, Status: status, Overdue: overdue, DueAt: time.Now()},
	}
}

func retentionSummary(t *testing.T, tasks fakeTasks, pf *analytics.Portfolio) (map[string]any, bool) {
	t.Helper()
	api := newAPI(fixture()).WithFeed(fakeFeed{pf}).WithTasks(tasks, func(context.Context) string { return "m" })
	rec := get(api.GetSummary, "/api/v1/analytics/summary")
	if rec.Code != 200 {
		t.Fatalf("статус %d: %s", rec.Code, rec.Body)
	}
	var got struct {
		Retention map[string]any `json:"retention"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	return got.Retention, got.Retention != nil
}

// Набор организаторов: в работе только клиент 9; высокий риск без работы — клиенты 10 и 5.
func TestSummaryRetention(t *testing.T) {
	pf := &analytics.Portfolio{Customers: []analytics.Customer{
		row(9, gen.High, 318910), row(10, gen.High, 20000), row(5, gen.High, 10000),
		row(2, gen.Medium, 5000), row(1, gen.Low, 100), row(3, gen.Lost, 0),
	}}
	got, ok := retentionSummary(t, fakeTasks{open: map[int64]analytics.TaskRef{9: ref(9, gen.InWork, false)}}, pf)
	if !ok {
		t.Fatal("блока retention нет")
	}
	want := map[string]float64{
		"tasksInWork": 1, "tasksContacted": 0, "tasksPostponed": 0, "tasksOverdue": 0,
		"customersInWork": 1, "lossInWorkCents": 318910,
		"highWithoutTask": 2, "highWithoutTaskLossCents": 30000,
		"mediumWithoutTask": 1, "mediumWithoutTaskLossCents": 5000,
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, ждали %v", k, got[k], v)
		}
	}
}

func TestSummaryRetentionStatusesAndMismatch(t *testing.T) {
	pf := &analytics.Portfolio{Customers: []analytics.Customer{row(1, gen.High, 100), row(2, gen.High, 200)}}
	other := ref(2, gen.Postponed, true)
	other.Email = "other@example.test" // у клиента 2 в снимке другой email: задача к нему не привязана
	open := map[int64]analytics.TaskRef{
		1: ref(1, gen.Contacted, true), 2: other, 7: ref(7, gen.InWork, true), // клиента 7 нет в снимке
	}
	got, _ := retentionSummary(t, fakeTasks{open: open}, pf)
	want := map[string]float64{
		"tasksInWork": 1, "tasksContacted": 1, "tasksPostponed": 1, "tasksOverdue": 3,
		"customersInWork": 1, "lossInWorkCents": 100, "highWithoutTask": 1, "highWithoutTaskLossCents": 200,
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, ждали %v", k, got[k], v)
		}
	}
}

// Ошибка чтения задач: сводка отдаётся, без блока retention.
func TestSummaryRetentionTasksError(t *testing.T) {
	pf := &analytics.Portfolio{Customers: []analytics.Customer{row(1, gen.High, 100)}}
	if _, ok := retentionSummary(t, fakeTasks{err: errors.New("база недоступна")}, pf); ok {
		t.Error("при ошибке задач блок retention не должен отдаваться")
	}
}

func TestSummaryRetentionNoSource(t *testing.T) {
	rec := get(newAPI(fixture()).GetSummary, "/api/v1/analytics/summary")
	var got map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if rec.Code != 200 || got["retention"] != nil {
		t.Fatalf("без источника задач: %d, retention=%v", rec.Code, got["retention"])
	}
}
