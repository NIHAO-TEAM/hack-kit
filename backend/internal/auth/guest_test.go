package auth_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"example.com/app/internal/apitest"
	"example.com/app/internal/auth"
	"example.com/app/internal/platform/api/gen"
)

func guestStand(t *testing.T) *apitest.Stand {
	t.Helper()
	return apitest.NewWithConfig(t, func(c *auth.Config) { c.GuestEnabled = true })
}

func postGuest(t *testing.T, s *apitest.Stand) *http.Response {
	t.Helper()
	resp, err := http.Post(s.URL+"/api/v1/auth/guest", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func guestToken(t *testing.T, s *apitest.Stand) string {
	t.Helper()
	resp := postGuest(t, s)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("вход гостя: статус %d", resp.StatusCode)
	}
	var res gen.LoginResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatal(err)
	}
	if res.User.Role != gen.Viewer || res.User.Login != "guest" || res.Token == nil || *res.Token == "" {
		t.Fatalf("ответ входа гостя: %+v", res)
	}
	return *res.Token
}

func TestGuestLoginDisabledByDefault(t *testing.T) {
	s := apitest.New(t)
	if resp := postGuest(t, s); resp.StatusCode != http.StatusNotFound {
		t.Errorf("гостевой вход выключен: статус %d, ждали 404", resp.StatusCode)
	}
	if getAuthConfig(t, s).GuestEnabled {
		t.Error("guestEnabled должен быть false без GUEST_LOGIN_ENABLED")
	}
}

func TestGuestLoginIssuesViewerSession(t *testing.T) {
	s := guestStand(t)
	if !getAuthConfig(t, s).GuestEnabled {
		t.Error("guestEnabled должен быть true")
	}
	resp := postGuest(t, s)
	if resp.StatusCode != http.StatusOK || sessionCookie(resp) == nil {
		t.Fatalf("статус %d, cookie %v", resp.StatusCode, sessionCookie(resp))
	}
	token := guestToken(t, s)
	me, err := s.WithBearer(t, token).GetMeWithResponse(t.Context())
	if err != nil || me.JSON200 == nil || me.JSON200.Role != gen.Viewer {
		t.Fatalf("getMe гостя: %v %+v", err, me)
	}
}

func TestGuestCannotLoginWithPassword(t *testing.T) {
	s := guestStand(t)
	guestToken(t, s)
	for _, pw := range []string{"!", "guest", "viewer", "pw-guest"} {
		if resp := login(t, s, "guest", pw); resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("пароль %q: статус %d, ждали 401", pw, resp.StatusCode)
		}
	}
}

func TestGuestReadsButNeverWrites(t *testing.T) {
	s := guestStand(t)
	token := guestToken(t, s)
	do := func(method, path string) int {
		req, err := http.NewRequest(method, s.URL+path, strings.NewReader("{}"))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}
	for _, path := range []string{
		"/api/v1/customers", "/api/v1/tasks", "/api/v1/analytics/summary",
		"/api/v1/model/settings", "/api/v1/source/status", "/api/v1/auth/me",
	} {
		// Данных на пустом стенде может не быть: важно, что права гостя не отказывают.
		if code := do(http.MethodGet, path); code == http.StatusUnauthorized || code == http.StatusForbidden {
			t.Errorf("GET %s: статус %d, гостю чтение разрешено", path, code)
		}
	}
	forbidden := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/audit"},
		{http.MethodGet, "/api/v1/model/sensitivity"},
		{http.MethodGet, "/api/v1/source/emulator/stream"},
		{http.MethodPost, "/api/v1/customers/1/task"},
		{http.MethodPatch, "/api/v1/tasks/1"},
		{http.MethodPost, "/api/v1/tasks/1/events"},
		{http.MethodPatch, "/api/v1/model/settings"},
		{http.MethodPut, "/api/v1/source/snapshot"},
		{http.MethodPost, "/api/v1/source/events"},
		{http.MethodPut, "/api/v1/source/polling"},
		{http.MethodPatch, "/api/v1/source/services/1"},
		{http.MethodPatch, "/api/v1/source/accounts/1"},
		{http.MethodPost, "/api/v1/source/emulator/tick"},
		{http.MethodPost, "/api/v1/source/emulator/actions"},
		{http.MethodPost, "/api/v1/source/emulator/stream"},
	}
	for _, f := range forbidden {
		if code := do(f.method, f.path); code != http.StatusForbidden {
			t.Errorf("%s %s: статус %d, ждали 403", f.method, f.path, code)
		}
	}
}

func TestGuestLoginRateLimit(t *testing.T) {
	s := guestStand(t)
	for i := 1; i <= 300; i++ {
		if resp := postGuest(t, s); resp.StatusCode != http.StatusOK {
			t.Fatalf("вход %d: статус %d", i, resp.StatusCode)
		}
	}
	resp := postGuest(t, s)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("вход сверх лимита: статус %d, ждали 429", resp.StatusCode)
	}
	if wait, err := strconv.Atoi(resp.Header.Get("Retry-After")); err != nil || wait <= 0 || wait > 60 {
		t.Errorf("Retry-After %q", resp.Header.Get("Retry-After"))
	}
	s.Clock.Advance(time.Minute)
	if resp := postGuest(t, s); resp.StatusCode != http.StatusOK {
		t.Errorf("после окна: статус %d", resp.StatusCode)
	}
}
