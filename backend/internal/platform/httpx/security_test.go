package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	h := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	got := rec.Header()
	csp := got.Get("Content-Security-Policy")
	if !strings.Contains(csp, "frame-ancestors 'none'") || !strings.Contains(csp, "script-src 'self';") {
		t.Errorf("CSP = %q", csp)
	}
	if strings.Contains(strings.Split(csp, "style-src")[0], "unsafe-inline") {
		t.Errorf("unsafe-inline в скриптах: %q", csp)
	}
	for k, want := range map[string]string{
		"Strict-Transport-Security": "max-age=",
		"X-Content-Type-Options":    "nosniff",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"X-Frame-Options":           "DENY",
	} {
		if !strings.Contains(got.Get(k), want) {
			t.Errorf("%s = %q, нужно %q", k, got.Get(k), want)
		}
	}
	if rec.Code != http.StatusTeapot {
		t.Errorf("статус %d", rec.Code)
	}
}
