package apitest_test

import (
	"context"
	"net/http"
	"testing"

	"example.com/app/internal/apitest"
)

// Пример теста эндпоинта через сгенерированный клиент.
func TestStandServesContractThroughClient(t *testing.T) {
	s := apitest.New(t)
	ctx := context.Background()

	health, err := s.Client.HealthzWithResponse(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if health.StatusCode() != http.StatusOK || health.JSON200 == nil || health.JSON200.Status != "ok" {
		t.Errorf("healthz: %d %s", health.StatusCode(), health.Body)
	}
	if health.HTTPResponse.Header.Get("X-Request-Id") == "" {
		t.Error("нет X-Request-Id")
	}

	summary, err := s.Client.GetSummaryWithResponse(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if summary.StatusCode() != http.StatusNotImplemented || summary.ApplicationproblemJSON501 == nil {
		t.Errorf("summary: %d %s", summary.StatusCode(), summary.Body)
	}
}

func TestStandsAreIsolated(t *testing.T) {
	a, b := apitest.New(t), apitest.New(t)
	if a.URL == b.URL {
		t.Error("два стенда делят один адрес")
	}
}
