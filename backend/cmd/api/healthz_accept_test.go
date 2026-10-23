package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthzAccept(t *testing.T) {
	tests := []struct {
		name   string
		accept string
	}{
		{"браузерный Accept", "text/html"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			req.Header.Set("Accept", tt.accept)
			rec := httptest.NewRecorder()
			newHandler().ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("статус %d, ждали %d", rec.Code, http.StatusOK)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q", ct)
			}
			var body map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if len(body) != 1 || body["status"] != "ok" {
				t.Errorf("тело %v", body)
			}
		})
	}
}
