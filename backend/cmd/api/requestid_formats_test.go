package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDFormats(t *testing.T) {
	const canonical = "f47ac10b-58cc-4372-8567-0e02b2c3d479"
	tests := []struct {
		name   string
		header string
	}{
		{"urn", "urn:uuid:" + canonical},
		{"фигурные скобки", "{" + canonical + "}"},
		{"urn в верхнем регистре", "urn:uuid:F47AC10B-58CC-4372-8567-0E02B2C3D479"},
		{"скобки в верхнем регистре", "{F47AC10B-58CC-4372-8567-0E02B2C3D479}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			req.Header.Set("X-Request-Id", tt.header)
			rec := httptest.NewRecorder()
			newHandler().ServeHTTP(rec, req)

			got := rec.Header().Get("X-Request-Id")
			if len(got) != 36 {
				t.Errorf("X-Request-Id = %q, длина %d, ждали 36", got, len(got))
			}
			if got != canonical {
				t.Errorf("X-Request-Id = %q, ждали %q", got, canonical)
			}
		})
	}
}
