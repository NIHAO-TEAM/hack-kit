package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestReadyHandler(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Lifecycle)
		want  int
	}{
		{"без проверок готов", func(*Lifecycle) {}, http.StatusOK},
		{"первый пересчёт не готов", func(l *Lifecycle) { l.Ready = func() bool { return false } }, http.StatusServiceUnavailable},
		{"пересчёт готов, база отвечает", func(l *Lifecycle) {
			l.Ready = func() bool { return true }
			l.Ping = func(context.Context) error { return nil }
		}, http.StatusOK},
		{"база недоступна", func(l *Lifecycle) {
			l.Ping = func(context.Context) error { return errors.New("down") }
		}, http.StatusServiceUnavailable},
		{"идёт остановка", func(l *Lifecycle) { l.BeginShutdown() }, http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewLifecycle()
			tt.setup(l)
			rec := httptest.NewRecorder()
			l.ReadyHandler(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			if rec.Code != tt.want {
				t.Fatalf("статус %d, ждали %d: %s", rec.Code, tt.want, rec.Body)
			}
		})
	}
}

func TestShutdownInflightMetric(t *testing.T) {
	l := NewLifecycle()
	started, release := make(chan struct{}), make(chan struct{})
	h := l.Track(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		started <- struct{}{}
		<-release
	}))
	var wg sync.WaitGroup
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
		}()
	}
	for range 3 {
		<-started
	}
	l.BeginShutdown()
	l.BeginShutdown() // повтор не перезаписывает число
	close(release)
	wg.Wait()

	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(l.Collector())
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	if len(families) != 1 || families[0].GetName() != "churn_shutdown_inflight" ||
		families[0].GetMetric()[0].GetGauge().GetValue() != 3 {
		t.Fatalf("метрика: %v", families)
	}
}
