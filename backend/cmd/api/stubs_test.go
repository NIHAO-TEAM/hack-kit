package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/app/internal/apitest"
	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/httpx"
)

// Операции, которые уже работают; все остальные — заглушки модулей.
var implemented = map[string]bool{
	"healthz": true, "version": true, "ping": true, "pingHead": true,
	"docs": true, "openapiYaml": true, "openapiJson": true,
	"login": true, "getMe": true, "logout": true, "getAuthConfig": true,
	"putSnapshot": true, "postSourceEvents": true, "getSourceStatus": true, "getEvents": true,
	"getModelSettings": true, "patchModelSettings": true, "getModelSensitivity": true,
	"listCustomers": true, "exportCustomers": true, "getCustomer": true,
	"getSummary": true, "getChurnChart": true, "getMetrics": true, "getMetricsAlias": true,
	"emulatorTick": true, "emulatorActions": true, "startEmulatorStream": true, "getEmulatorStream": true,
	"patchService": true, "patchAccount": true, "setSourcePolling": true, "listAudit": true,
	"takeCustomerTask": true, "listTasks": true, "listTaskAssignees": true, "getTask": true, "patchTask": true, "addTaskEvent": true,
}

func TestEveryUnimplementedPathAnswers501(t *testing.T) {
	swagger, err := gen.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	// Заглушки модулей закрыты входом: ходим под jury, которому открыты все пути.
	stand := apitest.New(t)
	login, err := stand.Client.LoginWithResponse(context.Background(), gen.LoginJSONRequestBody{Login: "jury", Password: apitest.Password("jury")})
	if err != nil || login.JSON200 == nil || login.JSON200.Token == nil {
		t.Fatalf("вход jury: %v %v", err, login)
	}
	h := stand.Handler
	token := *login.JSON200.Token
	checked, total := 0, 0
	for path, item := range swagger.Paths.Map() {
		for method, op := range item.Operations() {
			total++
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
			req.Header.Set("Authorization", "Bearer "+token)
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
	// Порог — на весь контракт: заглушек остаётся всё меньше, а обход должен охватывать контракт целиком.
	if total < 15 {
		t.Errorf("обойдено операций %d (из них заглушек %d), ждали не меньше 15", total, checked)
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
