package main

import (
	"encoding/json"
	"io"
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

func TestPing(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		wantStatus int
	}{
		{"GET отвечает pong", http.MethodGet, http.StatusOK},
		{"POST запрещён", http.MethodPost, http.StatusMethodNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			newHandler().ServeHTTP(rec, httptest.NewRequest(tt.method, "/ping", nil))

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

func TestHealthzHead(t *testing.T) {
	srv := httptest.NewServer(newHandler())
	defer srv.Close()

	resp, err := http.Head(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("статус %d, ждали 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if resp.Header.Get("X-Request-Id") == "" {
		t.Error("нет X-Request-Id")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != 0 {
		t.Errorf("тело не пустое: %q", body)
	}
}
