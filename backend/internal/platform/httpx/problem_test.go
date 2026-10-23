package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"example.com/app/internal/platform/httpx"
)

func TestWriteProblemFormat(t *testing.T) {
	rec := httptest.NewRecorder()
	rec.Header().Set("X-Request-Id", "5d6f6f0e-3c0e-4c4e-9a0e-1f2f3a4b5c6d")
	req := httptest.NewRequest(http.MethodGet, "/x", nil)

	httpx.WriteProblem(rec, req, http.StatusBadRequest, "плохой запрос",
		httpx.FieldError{Field: "limit", Message: "больше 200"})

	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("Content-Type = %q", got)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("статус = %d", rec.Code)
	}
	var p httpx.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Status != 400 || p.Title != "Bad Request" || p.Detail != "плохой запрос" {
		t.Errorf("тело: %+v", p)
	}
	if p.CorrelationID != "5d6f6f0e-3c0e-4c4e-9a0e-1f2f3a4b5c6d" {
		t.Errorf("correlation_id = %q", p.CorrelationID)
	}
	if len(p.Errors) != 1 || p.Errors[0].Field != "limit" {
		t.Errorf("errors: %+v", p.Errors)
	}
}

func TestNotImplemented(t *testing.T) {
	rec := httptest.NewRecorder()
	httpx.NotImplemented(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusNotImplemented {
		t.Errorf("статус = %d", rec.Code)
	}
}
