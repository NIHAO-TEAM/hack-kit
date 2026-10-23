package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVersionQuery(t *testing.T) {
	t.Setenv("APP_VERSION", "1.2.3")
	tests := []struct {
		name string
		url  string
	}{
		{"параметр x", "/version?x=1"},
		{"несколько параметров", "/version?x=1&y=2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			newHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.url, nil))

			if rec.Code != http.StatusOK {
				t.Fatalf("статус %d, ждали %d", rec.Code, http.StatusOK)
			}
			var body map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["version"] != "1.2.3" {
				t.Errorf("тело %v", body)
			}
		})
	}
}
