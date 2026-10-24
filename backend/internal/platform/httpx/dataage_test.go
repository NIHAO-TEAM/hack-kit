package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDataAge(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	var at time.Time
	h := DataAge(func() time.Time { return at }, func() time.Time { return now },
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	age := func(method, path string) string {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
		return rec.Header().Get("X-Data-Age")
	}
	if got := age(http.MethodGet, "/api/v1/customers"); got != "" {
		t.Fatalf("без расчёта заголовок %q", got)
	}
	at = now.Add(-42 * time.Second)
	if got := age(http.MethodGet, "/api/v1/customers"); got != "42" {
		t.Fatalf("возраст %q, ждали 42", got)
	}
	if got := age(http.MethodPost, "/api/v1/customers"); got != "" {
		t.Fatalf("на записи заголовок %q", got)
	}
	if got := age(http.MethodGet, "/healthz"); got != "" {
		t.Fatalf("вне /api/ заголовок %q", got)
	}
}
