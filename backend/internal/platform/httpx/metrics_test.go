package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// Метка route — шаблон маршрута: разные id дают один ряд, неизвестный путь — "unmatched".
func TestInstrumentUsesRoutePattern(t *testing.T) {
	requestDuration.Reset()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/customers/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	h := Instrument(mux)
	for _, p := range []string{"/api/v1/customers/1", "/api/v1/customers/2", "/nope/secret-123"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, p, nil))
	}
	reg := prometheus.NewRegistry()
	reg.MustRegister(Collectors()...)
	fams, err := reg.Gather()
	if err != nil || len(fams) != 1 {
		t.Fatalf("gather: %v, семейств %d", err, len(fams))
	}
	routes := map[string]uint64{}
	for _, m := range fams[0].GetMetric() {
		for _, l := range m.GetLabel() {
			if l.GetName() == "route" {
				routes[l.GetValue()] += m.GetHistogram().GetSampleCount()
			}
		}
	}
	if len(routes) != 2 || routes["GET /api/v1/customers/{id}"] != 2 || routes[unmatchedRoute] != 1 {
		t.Fatalf("route-метки: %v", routes)
	}
}
