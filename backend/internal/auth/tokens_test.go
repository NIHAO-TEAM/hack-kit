package auth_test

import (
	"context"
	"net/http"
	"testing"

	"example.com/app/internal/apitest"
)

func TestMetricsToken(t *testing.T) {
	s := apitest.New(t)
	ctx := context.Background()
	c := s.WithBearer(t, apitest.MetricsToken)

	for _, path := range []string{"/metrics", "/metics"} {
		resp := do(t, s, http.MethodGet, path, func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+apitest.MetricsToken) })
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			t.Errorf("%s с токеном сборщика: %d", path, resp.StatusCode)
		}
	}
	customers, err := c.ListCustomersWithResponse(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if customers.StatusCode() != http.StatusUnauthorized {
		t.Errorf("токен сборщика открыл /customers: %d", customers.StatusCode())
	}
	if resp := do(t, s, http.MethodPut, "/api/v1/source/snapshot", func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+apitest.MetricsToken) }); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("токен сборщика открыл snapshot: %d", resp.StatusCode)
	}
	if resp := do(t, s, http.MethodPost, "/api/v1/source/events", func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+apitest.MetricsToken) }); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("токен сборщика открыл events: %d", resp.StatusCode)
	}
}

func TestSourceTokenOpensOnlySnapshot(t *testing.T) {
	s := apitest.New(t)
	bearer := func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+apitest.SourceToken) }
	if resp := do(t, s, http.MethodPut, "/api/v1/source/snapshot", bearer); resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		t.Errorf("snapshot с сервисным токеном: %d", resp.StatusCode)
	}
	if resp := do(t, s, http.MethodPost, "/api/v1/source/events", bearer); resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		t.Errorf("events с сервисным токеном: %d", resp.StatusCode)
	}
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/customers"}, {http.MethodGet, "/api/v1/analytics/summary"},
		{http.MethodPatch, "/api/v1/model/settings"}, {http.MethodGet, "/metrics"}, {http.MethodGet, "/api/v1/auth/me"},
		{http.MethodPost, "/api/v1/source/emulator/tick"},
	} {
		if resp := do(t, s, tc.method, tc.path, bearer); resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("сервисный токен открыл %s %s: %d", tc.method, tc.path, resp.StatusCode)
		}
	}
}

func TestEmptyServiceTokensNeverMatch(t *testing.T) {
	// Пустой токен в настройках не должен совпадать с пустым Bearer.
	s := apitest.New(t)
	resp := do(t, s, http.MethodGet, "/metrics", func(r *http.Request) { r.Header.Set("Authorization", "Bearer ") })
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("пустой Bearer: %d", resp.StatusCode)
	}
}

// Профили процесса открывает только токен метрик: ни сессия любой роли, ни другой токен, ни анонимный запрос.
func TestPprofOnlyWithMetricsToken(t *testing.T) {
	s := apitest.New(t)
	get := func(mod func(*http.Request)) int {
		return do(t, s, http.MethodGet, "/debug/pprof/goroutine?debug=1", mod).StatusCode
	}
	if code := get(nil); code != http.StatusUnauthorized {
		t.Errorf("без токена: %d, ждали 401", code)
	}
	if code := get(func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+apitest.SourceToken) }); code != http.StatusUnauthorized {
		t.Errorf("с токеном источника: %d, ждали 401", code)
	}
	if code := get(func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+apitest.MetricsToken) }); code != http.StatusOK {
		t.Errorf("с токеном метрик: %d, ждали 200", code)
	}
}
