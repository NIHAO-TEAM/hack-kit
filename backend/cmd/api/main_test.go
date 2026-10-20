package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
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

func TestRequestID(t *testing.T) {
	const given = "f47ac10b-58cc-4372-8567-0e02b2c3d479"
	tests := []struct {
		name       string
		method     string
		header     string
		wantStatus int
		wantSame   bool
		want       string
	}{
		{"заголовка нет", http.MethodGet, "", http.StatusOK, false, ""},
		{"корректный UUID возвращается", http.MethodGet, given, http.StatusOK, true, ""},
		{"верхний регистр нормализуется", http.MethodGet, strings.ToUpper(given), http.StatusOK, false, given},
		{"мусор заменяется", http.MethodGet, "not-a-uuid", http.StatusOK, false, ""},
		{"405 тоже с заголовком", http.MethodPost, "", http.StatusMethodNotAllowed, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/healthz", nil)
			if tt.header != "" {
				req.Header.Set("X-Request-Id", tt.header)
			}
			rec := httptest.NewRecorder()
			newHandler().ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("статус %d, ждали %d", rec.Code, tt.wantStatus)
			}
			got := rec.Header().Get("X-Request-Id")
			if _, err := uuid.Parse(got); err != nil {
				t.Fatalf("X-Request-Id = %q, не UUID: %v", got, err)
			}
			if tt.wantSame && got != tt.header {
				t.Errorf("X-Request-Id = %q, ждали %q", got, tt.header)
			}
			if tt.want != "" && got != tt.want {
				t.Errorf("X-Request-Id = %q, ждали %q", got, tt.want)
			}
			if !tt.wantSame && tt.want == "" && tt.header != "" && got == tt.header {
				t.Errorf("X-Request-Id = %q, ждали новый", got)
			}
		})
	}
}
