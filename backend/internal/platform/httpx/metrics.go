package httpx

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// unmatchedRoute — значение route, когда маршрутизатор не нашёл шаблон: сырой путь в метку не попадает.
const unmatchedRoute = "unmatched"

var requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "churn_http_request_duration_seconds",
	Help:    "Длительность HTTP-запроса (секунды); route — шаблон маршрута, не сырой путь.",
	Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
}, []string{"route", "method", "code"})

// Collectors — метрики HTTP для /metrics.
func Collectors() []prometheus.Collector { return []prometheus.Collector{requestDuration} }

// Instrument меряет длительность запросов к mux. Шаблон маршрута берётся из r.Pattern после
// обработки, поэтому mux должен получать тот же *http.Request, что и обёртка.
func Instrument(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, code: http.StatusOK}
		mux.ServeHTTP(sw, r)
		route := r.Pattern
		if route == "" {
			route = unmatchedRoute
		}
		requestDuration.WithLabelValues(route, r.Method, strconv.Itoa(sw.code)).Observe(time.Since(start).Seconds())
	})
}

type statusWriter struct {
	http.ResponseWriter
	code    int
	written bool
}

func (s *statusWriter) WriteHeader(code int) {
	if !s.written {
		s.code, s.written = code, true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	s.written = true
	return s.ResponseWriter.Write(b)
}

// Flush нужен потоковым ответам (события).
func (s *statusWriter) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap отдаёт исходный ResponseWriter для http.ResponseController.
func (s *statusWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }
