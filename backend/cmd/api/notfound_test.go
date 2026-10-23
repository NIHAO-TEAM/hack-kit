package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNotFound(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"неизвестный путь", "/nope"},
		{"подпуть известного маршрута", "/healthz/extra"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			newHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if rec.Code != http.StatusNotFound {
				t.Fatalf("статус %d, ждали %d", rec.Code, http.StatusNotFound)
			}
			if rec.Header().Get("X-Request-Id") == "" {
				t.Fatal("нет заголовка X-Request-Id")
			}
		})
	}
}
