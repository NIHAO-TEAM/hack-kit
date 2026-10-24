package httpx

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestLimitShedsFastAndServesAccepted(t *testing.T) {
	const limit, total = 4, 40
	release := make(chan struct{})
	var served atomic.Int32
	slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		served.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	h := Limit(LimitConfig{Read: limit, Write: 1, Ingest: 1, Wait: 50 * time.Millisecond}, slow)
	before := testutil.ToFloat64(shed.WithLabelValues(ClassRead))

	codes := make([]int, total)
	took := make([]time.Duration, total)
	var wg sync.WaitGroup
	for i := range total {
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/customers", nil))
			codes[i], took[i] = rec.Code, time.Since(start)
			if rec.Code == http.StatusServiceUnavailable && rec.Header().Get("Retry-After") != "1" {
				t.Errorf("503 без Retry-After: 1")
			}
		}()
	}
	// Отпускаем медленные запросы, только когда все лишние уже получили отказ: без гонки с таймером.
	deadline := time.Now().Add(5 * time.Second)
	for testutil.ToFloat64(shed.WithLabelValues(ClassRead))-before < total-limit && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	close(release)
	wg.Wait()

	var ok, rejected int
	for i, c := range codes {
		switch c {
		case http.StatusOK:
			ok++
		case http.StatusServiceUnavailable:
			rejected++
			if took[i] > 150*time.Millisecond {
				t.Errorf("отказ через %v, ждём не дольше 150 мс", took[i])
			}
		default:
			t.Errorf("неожиданный код %d", c)
		}
	}
	if ok != limit || rejected != total-limit {
		t.Fatalf("принято %d, отказано %d; ждём %d и %d", ok, rejected, limit, total-limit)
	}
	if got := testutil.ToFloat64(shed.WithLabelValues(ClassRead)) - before; got != total-limit {
		t.Fatalf("churn_http_shed_total{class=read} вырос на %v, ждём %d", got, total-limit)
	}
	if v := testutil.ToFloat64(inflight.WithLabelValues(ClassRead)); v != 0 {
		t.Fatalf("после запросов в работе %v, ждём 0", v)
	}
}

func TestLimitClasses(t *testing.T) {
	cases := []struct {
		method, path, want string
	}{
		{http.MethodGet, "/healthz", ""},
		{http.MethodGet, "/readyz", ""},
		{http.MethodGet, "/metrics", ""},
		{http.MethodGet, "/queue", ""},
		{http.MethodGet, "/api/v1/events", ""},
		{http.MethodGet, "/api/v1/docs/", ""},
		{http.MethodGet, "/api/v1/customers", ClassRead},
		{http.MethodGet, "/api/v1/source/status", ClassRead},
		{http.MethodPatch, "/api/v1/model/settings", ClassWrite},
		{http.MethodPost, "/api/v1/auth/login", ClassWrite},
		{http.MethodPut, "/api/v1/source/snapshot", ClassIngest},
		{http.MethodPost, "/api/v1/source/emulator/stream", ClassIngest},
	}
	for _, c := range cases {
		if got := limitClass(httptest.NewRequest(c.method, c.path, nil)); got != c.want {
			t.Errorf("%s %s: класс %q, ждём %q", c.method, c.path, got, c.want)
		}
	}
}

func TestLimitClassesDoNotBlockEachOther(t *testing.T) {
	release := make(chan struct{})
	block := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			<-release
		}
		w.WriteHeader(http.StatusOK)
	})
	h := Limit(LimitConfig{Read: 1, Write: 1, Ingest: 1, Wait: 10 * time.Millisecond}, block)
	done := make(chan struct{})
	go func() {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/customers", nil))
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/v1/source/snapshot", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("приём при занятом чтении: %d, ждём 200", rec.Code)
	}
	close(release)
	<-done
}
