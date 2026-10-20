package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthz(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		wantStatus int
	}{
		{"GET отвечает ok", http.MethodGet, http.StatusOK},
		{"POST запрещён", http.MethodPost, http.StatusMethodNotAllowed},
		{"DELETE запрещён", http.MethodDelete, http.StatusMethodNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			newMux().ServeHTTP(rec, httptest.NewRequest(tt.method, "/healthz", nil))

			if rec.Code != tt.wantStatus {
				t.Fatalf("статус %d, ждали %d", rec.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusMethodNotAllowed {
				if !strings.Contains(rec.Header().Get("Allow"), http.MethodGet) {
					t.Errorf("Allow = %q, ждали GET", rec.Header().Get("Allow"))
				}
				return
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q", ct)
			}
			var body map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["status"] != "ok" {
				t.Errorf("тело %v", body)
			}
		})
	}
}

func TestVersion(t *testing.T) {
	tests := []struct {
		name        string
		env         string
		method      string
		wantStatus  int
		wantVersion string
	}{
		{"версия не задана", "", http.MethodGet, http.StatusOK, "dev"},
		{"версия задана", "1.2.3", http.MethodGet, http.StatusOK, "1.2.3"},
		{"POST запрещён", "1.2.3", http.MethodPost, http.StatusMethodNotAllowed, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("APP_VERSION", tt.env)
			rec := httptest.NewRecorder()
			newMux().ServeHTTP(rec, httptest.NewRequest(tt.method, "/version", nil))

			if rec.Code != tt.wantStatus {
				t.Fatalf("статус %d, ждали %d", rec.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusMethodNotAllowed {
				if !strings.Contains(rec.Header().Get("Allow"), http.MethodGet) {
					t.Errorf("Allow = %q, ждали GET", rec.Header().Get("Allow"))
				}
				return
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q", ct)
			}
			var body map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["version"] != tt.wantVersion {
				t.Errorf("тело %v", body)
			}
		})
	}
}
