package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPingQuery(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"параметр x", "/ping?x=1"},
		{"метка времени t", "/ping?t=1700000000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			newHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.url, nil))

			if rec.Code != http.StatusOK {
				t.Fatalf("статус %d, ждали %d", rec.Code, http.StatusOK)
			}
			if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
				t.Errorf("Cache-Control = %q", cc)
			}
			var body map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if len(body) != 1 || body["ping"] != "pong" {
				t.Errorf("тело %v", body)
			}
		})
	}
}
