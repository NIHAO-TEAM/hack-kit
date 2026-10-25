package auth_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"example.com/app/internal/apitest"
)

func do(t *testing.T, s *apitest.Stand, method, path string, mod func(*http.Request)) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, s.URL+path, strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if mod != nil {
		mod(req)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func TestCSRFOnlyForCookieSessions(t *testing.T) {
	s := apitest.New(t)
	lg := login(t, s, "analyst", apitest.Password("analyst"))
	cookie := sessionCookie(lg)
	meReq := do(t, s, http.MethodGet, "/api/v1/auth/me", func(r *http.Request) { r.AddCookie(cookie) })
	csrf := meReq.Header.Get("X-CSRF-Token")
	if csrf == "" {
		t.Fatal("me не вернул X-CSRF-Token")
	}
	// Токен Bearer — тот же, что в cookie.
	bearer := cookie.Value

	t.Run("cookie без X-CSRF-Token — 403", func(t *testing.T) {
		resp := do(t, s, http.MethodPatch, "/api/v1/model/settings", func(r *http.Request) { r.AddCookie(cookie) })
		if resp.StatusCode != http.StatusForbidden || resp.Header.Get("Content-Type") != "application/problem+json" {
			t.Errorf("статус %d, %s", resp.StatusCode, resp.Header.Get("Content-Type"))
		}
	})
	t.Run("cookie с чужим X-CSRF-Token — 403", func(t *testing.T) {
		resp := do(t, s, http.MethodPatch, "/api/v1/model/settings", func(r *http.Request) {
			r.AddCookie(cookie)
			r.Header.Set("X-CSRF-Token", "wrong")
		})
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("статус %d", resp.StatusCode)
		}
	})
	t.Run("cookie с верным X-CSRF-Token проходит", func(t *testing.T) {
		resp := do(t, s, http.MethodPatch, "/api/v1/model/settings", func(r *http.Request) {
			r.AddCookie(cookie)
			r.Header.Set("X-CSRF-Token", csrf)
		})
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
			t.Errorf("статус %d", resp.StatusCode)
		}
	})
	t.Run("Bearer без X-CSRF-Token проходит", func(t *testing.T) {
		resp := do(t, s, http.MethodPatch, "/api/v1/model/settings", func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer "+bearer)
		})
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
			t.Errorf("статус %d", resp.StatusCode)
		}
	})
	t.Run("GET с cookie не требует токена", func(t *testing.T) {
		resp := do(t, s, http.MethodGet, "/api/v1/auth/me", func(r *http.Request) { r.AddCookie(cookie) })
		if resp.StatusCode != http.StatusOK {
			t.Errorf("статус %d", resp.StatusCode)
		}
	})
}

func TestNoSessionMeans401(t *testing.T) {
	s := apitest.New(t)
	for _, path := range []string{"/api/v1/customers", "/api/v1/customers/1", "/api/v1/analytics/summary",
		"/api/v1/model/settings", "/api/v1/source/status", "/api/v1/events", "/api/v1/auth/me", "/metrics", "/metics"} {
		resp := do(t, s, http.MethodGet, path, nil)
		if resp.StatusCode != http.StatusUnauthorized || resp.Header.Get("Content-Type") != "application/problem+json" {
			t.Errorf("%s: статус %d, %s", path, resp.StatusCode, resp.Header.Get("Content-Type"))
		}
	}
	// Мусорный токен — тот же 401.
	resp := do(t, s, http.MethodGet, "/api/v1/customers", func(r *http.Request) { r.Header.Set("Authorization", "Bearer junk") })
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("мусорный токен: %d", resp.StatusCode)
	}
}

func TestPublicPathsOpen(t *testing.T) {
	s := apitest.New(t)
	for _, path := range []string{"/api/v1/docs", "/api/v1/openapi.yml", "/api/v1/openapi.json", "/healthz", "/version", "/ping", "/"} {
		if resp := do(t, s, http.MethodGet, path, nil); resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			t.Errorf("%s закрыт: %d", path, resp.StatusCode)
		}
	}
}

// Маркетолог и аналитик — один пользователь (ADR-0001, дополнение 25.10): модель меняет и тот и другой.
func TestMarketerAndAnalystShareRights(t *testing.T) {
	s := apitest.New(t)
	ctx := context.Background()
	for _, role := range []string{"marketer", "analyst"} {
		c := s.As(t, role)
		get, err := c.GetModelSettingsWithResponse(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if get.StatusCode() == http.StatusForbidden || get.StatusCode() == http.StatusUnauthorized {
			t.Errorf("%s: GET model/settings закрыт: %d", role, get.StatusCode())
		}
		patch, err := c.PatchModelSettingsWithBody(ctx, nil, "application/json", strings.NewReader("{}"))
		if err != nil {
			t.Fatal(err)
		}
		_ = patch.Body.Close()
		if patch.StatusCode == http.StatusForbidden || patch.StatusCode == http.StatusUnauthorized {
			t.Errorf("%s: PATCH model/settings закрыт: %d", role, patch.StatusCode)
		}
	}
}

func TestModelSensitivityAccess(t *testing.T) {
	s := apitest.New(t)
	if resp := do(t, s, http.MethodGet, "/api/v1/model/sensitivity", nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("без сессии: %d, ждали 401", resp.StatusCode)
	}
	for _, role := range []string{"marketer", "analyst", "jury"} {
		c := sessionCookie(login(t, s, role, apitest.Password(role)))
		resp := do(t, s, http.MethodGet, "/api/v1/model/sensitivity", func(r *http.Request) { r.AddCookie(c) })
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
			t.Errorf("%s: %d, доступ должен быть", role, resp.StatusCode)
		}
	}
}
