package source

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeIDs — окно eventId в памяти; commit закрепляет занятые id, rollback освобождает.
type fakeIDs struct {
	mu      sync.Mutex
	seen    map[string]bool
	claims  int
	commits int
	backs   int
}

func newFakeIDs() *fakeIDs { return &fakeIDs{seen: map[string]bool{}} }

type fakeTx struct {
	f   *fakeIDs
	own []string
}

func (f *fakeIDs) claim(_ context.Context, ids []string, _ time.Time) ([]string, idTx, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claims++
	tx := &fakeTx{f: f}
	var fresh []string
	for _, id := range ids {
		if !f.seen[id] {
			f.seen[id] = true
			fresh = append(fresh, id)
			tx.own = append(tx.own, id)
		}
	}
	return fresh, tx, nil
}

func (f *fakeIDs) clean(context.Context, time.Time) error { return nil }

func (t *fakeTx) Commit(context.Context) error {
	t.f.mu.Lock()
	t.f.commits++
	t.f.mu.Unlock()
	return nil
}

func (t *fakeTx) Rollback(context.Context) error {
	t.f.mu.Lock()
	defer t.f.mu.Unlock()
	t.f.backs++
	for _, id := range t.own {
		delete(t.f.seen, id)
	}
	return nil
}

// testIntake — приём с писателем, который не запущен: очередь только копится. Базы нет.
func testIntake(ids eventIDStore, queue int) *eventIntake {
	w := newEventWriter(nil, newOpenIngestState(), time.Now, func(int64) {}, func([]ingestResult) {})
	w.in = make(chan ingestItem, queue)
	return &eventIntake{
		w: w, ids: ids, now: time.Now, wait: 50 * time.Millisecond,
		version: func(context.Context) (int64, bool, error) { return 7, true, nil },
	}
}

func postEvents(a *SourceAPI, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/source/events", strings.NewReader(body))
	a.PostSourceEvents(rec, req)
	return rec
}

func TestPostSourceEventsStatuses(t *testing.T) {
	a := &SourceAPI{intake: testIntake(newFakeIDs(), 16)}
	a.intakeOnce.Do(func() {})

	ok := `{"eventId":"e1","kind":"top_up","accountId":1,"balance":500}`
	if rec := postEvents(a, ok); rec.Code != http.StatusAccepted {
		t.Errorf("годное событие: %d %s", rec.Code, rec.Body)
	}
	if rec := postEvents(a, `[{"eventId":`); rec.Code != http.StatusBadRequest {
		t.Errorf("битый массив: %d", rec.Code)
	}
	if rec := postEvents(a, strings.Repeat(ok+"\n", eventsMaxCount+1)); rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("больше 5000 событий: %d", rec.Code)
	}
}

func TestPostSourceEventsBodyOverLimit(t *testing.T) {
	a := &SourceAPI{intake: testIntake(newFakeIDs(), 16)}
	a.intakeOnce.Do(func() {})
	// 4 800 строк (меньше предела числа событий), а всё тело больше 2 МБ.
	line := `{"eventId":"e","pad":"` + strings.Repeat("x", 450) + "\"}\n"
	body := strings.Repeat(line, 4800)
	if len(body) <= eventsMaxBytes {
		t.Fatalf("тело %d байт, ждали больше %d", len(body), eventsMaxBytes)
	}
	rec := postEvents(a, body)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("тело больше 2 МБ: %d %s", rec.Code, rec.Body)
	}
}
