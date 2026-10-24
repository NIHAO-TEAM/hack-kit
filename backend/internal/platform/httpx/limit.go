package httpx

import (
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Класс запроса: у каждого свой предел, чтобы залп чтения не блокировал приём событий и наоборот.
const (
	ClassRead   = "read"
	ClassWrite  = "write"
	ClassIngest = "ingest"
)

var (
	inflight = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "churn_http_inflight",
		Help: "Запросов в работе по классу предела (read, write, ingest).",
	}, []string{"class"})
	shed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "churn_http_shed_total",
		Help: "Запросов, получивших 503 из-за перегрузки, по классу предела.",
	}, []string{"class"})
)

// LimitCollectors — метрики предела для /metrics.
func LimitCollectors() []prometheus.Collector { return []prometheus.Collector{inflight, shed} }

// LimitConfig — пределы одновременных запросов по классам и сколько ждать места.
type LimitConfig struct {
	Read, Write, Ingest int
	Wait                time.Duration
}

// LimitConfigFromEnv: HTTP_MAX_INFLIGHT_READ (по умолчанию 8 × GOMAXPROCS), HTTP_MAX_INFLIGHT_WRITE
// (2 × GOMAXPROCS), HTTP_MAX_INFLIGHT_INGEST (2), HTTP_MAX_QUEUE_WAIT (100ms). Ноль — без предела.
func LimitConfigFromEnv() LimitConfig {
	n := runtime.GOMAXPROCS(0)
	c := LimitConfig{Read: 8 * n, Write: 2 * n, Ingest: 2, Wait: 100 * time.Millisecond}
	envInt := func(key string, dst *int) {
		if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v >= 0 {
			*dst = v
		}
	}
	envInt("HTTP_MAX_INFLIGHT_READ", &c.Read)
	envInt("HTTP_MAX_INFLIGHT_WRITE", &c.Write)
	envInt("HTTP_MAX_INFLIGHT_INGEST", &c.Ingest)
	if d, err := time.ParseDuration(os.Getenv("HTTP_MAX_QUEUE_WAIT")); err == nil && d >= 0 {
		c.Wait = d
	}
	return c
}

// Limit отвечает 503 с Retry-After на запросы сверх предела своего класса, если место не
// освободилось за cfg.Wait: лишние получают быстрый отказ, принятые обслуживаются вовремя,
// память не копится. Служебные пути, поток событий и статика фронта предела не имеют.
func Limit(cfg LimitConfig, next http.Handler) http.Handler {
	sems := map[string]chan struct{}{}
	for class, n := range map[string]int{ClassRead: cfg.Read, ClassWrite: cfg.Write, ClassIngest: cfg.Ingest} {
		if n > 0 {
			sems[class] = make(chan struct{}, n)
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		class := limitClass(r)
		sem := sems[class]
		if sem == nil {
			next.ServeHTTP(w, r)
			return
		}
		select {
		case sem <- struct{}{}:
		default:
			t := time.NewTimer(cfg.Wait)
			select {
			case sem <- struct{}{}:
				t.Stop()
			case <-t.C:
				shed.WithLabelValues(class).Inc()
				w.Header().Set("Retry-After", "1")
				WriteProblem(w, r, http.StatusServiceUnavailable, "Сервис перегружен, повторите через секунду")
				return
			case <-r.Context().Done():
				t.Stop()
				return
			}
		}
		g := inflight.WithLabelValues(class)
		g.Inc()
		defer func() {
			g.Dec()
			<-sem
		}()
		next.ServeHTTP(w, r)
	})
}

// limitClass — класс запроса; пустая строка — вне предела.
func limitClass(r *http.Request) string {
	p := r.URL.Path
	switch {
	case !strings.HasPrefix(p, "/api/"):
		return "" // /healthz, /readyz, /metrics, /debug/pprof, статика фронта
	case p == "/api/v1/events", strings.HasPrefix(p, "/api/v1/docs"):
		return "" // долгий поток SSE и Swagger
	case strings.HasPrefix(p, "/api/v1/source/") && r.Method != http.MethodGet && r.Method != http.MethodHead:
		return ClassIngest
	case r.Method == http.MethodGet || r.Method == http.MethodHead:
		return ClassRead
	default:
		return ClassWrite
	}
}
