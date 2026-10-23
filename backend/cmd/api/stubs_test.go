package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/httpx"
)

// Операции, которые уже работают; все остальные — заглушки модулей.
var implemented = map[string]bool{
	"healthz": true, "version": true, "ping": true, "pingHead": true,
	"docs": true, "openapiYaml": true, "openapiJson": true,
}

func TestEveryUnimplementedPathAnswers501(t *testing.T) {
	swagger, err := gen.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	h := newHandler()
	checked := 0
	for path, item := range swagger.Paths.Map() {
		for method, op := range item.Operations() {
			// Встроенная спека хранит operationId с заглавной буквы.
			if implemented[strings.ToLower(op.OperationID[:1])+op.OperationID[1:]] {
				continue
			}
			url := strings.NewReplacer("{id}", "1").Replace(path)
			var body *strings.Reader
			if op.RequestBody != nil {
				body = strings.NewReader("{}")
			} else {
				body = strings.NewReader("")
			}
			req := httptest.NewRequest(method, url, body)
			if op.RequestBody != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			name := method + " " + path
			if rec.Code != http.StatusNotImplemented {
				t.Errorf("%s: статус %d, ждали 501", name, rec.Code)
				continue
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
				t.Errorf("%s: Content-Type %q", name, ct)
			}
			var p httpx.Problem
			if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
				t.Errorf("%s: тело не JSON: %v", name, err)
				continue
			}
			if p.Status != 501 || p.CorrelationID == "" || p.CorrelationID != rec.Header().Get("X-Request-Id") {
				t.Errorf("%s: problem %+v, X-Request-Id %q", name, p, rec.Header().Get("X-Request-Id"))
			}
			checked++
		}
	}
	if checked < 20 {
		t.Errorf("проверено операций %d, ждали не меньше 20", checked)
	}
}

func TestMandatoryCasePathsAreInContract(t *testing.T) {
	swagger, err := gen.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ method, path string }{
		{"POST", "/api/v1/auth/login"}, {"GET", "/api/v1/auth/me"}, {"POST", "/api/v1/auth/logout"},
		{"GET", "/api/v1/customers"}, {"GET", "/api/v1/customers/{id}"},
		{"GET", "/api/v1/analytics/summary"},
		{"GET", "/api/v1/model/settings"}, {"PATCH", "/api/v1/model/settings"},
		{"PUT", "/api/v1/source/snapshot"}, {"GET", "/metrics"}, {"GET", "/metics"},
		{"GET", "/api/v1/docs"},
	} {
		item := swagger.Paths.Find(c.path)
		if item == nil || item.GetOperation(c.method) == nil {
			t.Errorf("в контракте нет %s %s", c.method, c.path)
		}
	}
}
