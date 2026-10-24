package httpx

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// readyCheckTimeout — сколько /readyz ждёт ответа базы.
const readyCheckTimeout = time.Second

// Lifecycle — готовность и остановка процесса. Под выкат его использует cmd/api:
// /readyz отвечает 503, пока нет первого пересчёта и во время остановки; запросы в работе
// считаются, а контекст долгих потоков (SSE) отменяется, когда сервер начинает останавливаться.
type Lifecycle struct {
	// Ready сообщает, что первый пересчёт готов; nil — всегда готов.
	Ready func() bool
	// Ping проверяет базу; nil — не проверяется.
	Ping func(ctx context.Context) error

	stopping atomic.Bool
	inflight atomic.Int64
	atSignal atomic.Int64

	once          sync.Once
	streams       context.Context
	cancelStreams context.CancelFunc
}

// NewLifecycle создаёт состояние «работаю».
func NewLifecycle() *Lifecycle {
	l := &Lifecycle{}
	l.streams, l.cancelStreams = context.WithCancel(context.Background())
	return l
}

// BeginShutdown переводит процесс в «останавливаюсь»: /readyz отвечает 503, число запросов
// в работе запоминается для метрики. Повторный вызов ничего не меняет.
func (l *Lifecycle) BeginShutdown() {
	l.once.Do(func() {
		l.atSignal.Store(l.inflight.Load())
		l.stopping.Store(true)
	})
}

// Stopping — остановка началась.
func (l *Lifecycle) Stopping() bool { return l.stopping.Load() }

// InflightAtShutdown — запросов в работе на момент сигнала (0 до сигнала).
func (l *Lifecycle) InflightAtShutdown() int64 { return l.atSignal.Load() }

// CancelStreams отменяет контекст долгих потоков; подключается через srv.RegisterOnShutdown.
func (l *Lifecycle) CancelStreams() { l.cancelStreams() }

// BaseContext отдаётся http.Server.BaseContext: контекст каждого запроса наследует отмену потоков.
func (l *Lifecycle) BaseContext() context.Context { return l.streams }

// Track считает запросы в работе. Сам /readyz проходит мимо него.
func (l *Lifecycle) Track(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l.inflight.Add(1)
		defer l.inflight.Add(-1)
		next.ServeHTTP(w, r)
	})
}

// ReadyHandler — GET /readyz: 200, когда первый пересчёт готов, база отвечает и остановка не начата.
func (l *Lifecycle) ReadyHandler(w http.ResponseWriter, r *http.Request) {
	status, reason := http.StatusOK, "ok"
	switch {
	case l.Stopping():
		status, reason = http.StatusServiceUnavailable, "stopping"
	case l.Ready != nil && !l.Ready():
		status, reason = http.StatusServiceUnavailable, "warming up"
	case l.Ping != nil:
		ctx, cancel := context.WithTimeout(r.Context(), readyCheckTimeout)
		err := l.Ping(ctx)
		cancel()
		if err != nil {
			status, reason = http.StatusServiceUnavailable, "database unavailable"
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": reason})
}

var descInflight = prometheus.NewDesc("churn_shutdown_inflight",
	"Запросов в работе на момент сигнала остановки (0, пока остановка не начиналась).", nil, nil)

// Collector — метрика churn_shutdown_inflight для /metrics.
func (l *Lifecycle) Collector() prometheus.Collector { return inflightCollector{l} }

type inflightCollector struct{ l *Lifecycle }

func (inflightCollector) Describe(ch chan<- *prometheus.Desc) { ch <- descInflight }

func (c inflightCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(descInflight, prometheus.GaugeValue, float64(c.l.InflightAtShutdown()))
}
