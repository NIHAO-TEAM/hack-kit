package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"example.com/app/internal/apitest"
	"example.com/app/internal/platform/api/gen"
)

// Маршрут выгрузки — литерал и не путается с /customers/{id}: без сессии 401, с сессией не 400/404.
func TestExportRouteIsNotCustomerID(t *testing.T) {
	stand := apitest.New(t)
	h := stand.Handler

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/customers/export.csv", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("без сессии: статус %d, ждали 401", rec.Code)
	}

	login, err := stand.Client.LoginWithResponse(context.Background(), gen.LoginJSONRequestBody{Login: "marketer", Password: apitest.Password("marketer")})
	if err != nil || login.JSON200 == nil || login.JSON200.Token == nil {
		t.Fatalf("вход marketer: %v %v", err, login)
	}
	req := httptest.NewRequest("GET", "/api/v1/customers/export.csv", nil)
	req.Header.Set("Authorization", "Bearer "+*login.JSON200.Token)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusBadRequest || rec.Code == http.StatusNotFound {
		t.Errorf("маршрут разобран как /customers/{id}: статус %d, тело %s", rec.Code, rec.Body)
	}
}
