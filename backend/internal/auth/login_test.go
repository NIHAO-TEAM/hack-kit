package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"example.com/app/internal/apitest"
	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/httpx"
)

func login(t *testing.T, s *apitest.Stand, user, password string) *http.Response {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"login": user, "password": password})
	resp, err := http.Post(s.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func sessionCookie(resp *http.Response) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == "session" {
			return c
		}
	}
	return nil
}

func TestLoginCorrectPassword(t *testing.T) {
	s := apitest.New(t)
	resp := login(t, s, "analyst", apitest.Password("analyst"))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("статус %d", resp.StatusCode)
	}
	var res gen.LoginResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatal(err)
	}
	if res.Token == nil || *res.Token == "" || res.CsrfToken == nil || *res.CsrfToken == "" {
		t.Fatalf("нет token или csrfToken: %+v", res)
	}
	if res.User.Login != "analyst" || res.User.Role != gen.Analyst {
		t.Errorf("пользователь %+v", res.User)
	}
	c := sessionCookie(resp)
	if c == nil || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Value != *res.Token {
		t.Errorf("cookie сессии: %+v", c)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	s := apitest.New(t)
	for name, tc := range map[string]struct{ login, password string }{
		"неверный пароль":      {"analyst", "wrong"},
		"несуществующий логин": {"nobody", "wrong"},
	} {
		t.Run(name, func(t *testing.T) {
			resp := login(t, s, tc.login, tc.password)
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("статус %d", resp.StatusCode)
			}
			if ct := resp.Header.Get("Content-Type"); ct != "application/problem+json" {
				t.Errorf("Content-Type %q", ct)
			}
			if sessionCookie(resp) != nil {
				t.Error("cookie выдана при неверном пароле")
			}
			var p httpx.Problem
			_ = json.NewDecoder(resp.Body).Decode(&p)
			if strings.Contains(strings.ToLower(p.Detail), "пароль неверн") || p.Status != 401 {
				t.Errorf("problem %+v", p)
			}
		})
	}
}

func TestLoginBadBody(t *testing.T) {
	s := apitest.New(t)
	resp, err := http.Post(s.URL+"/api/v1/auth/login", "application/json", strings.NewReader(`{"login":"jury"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("статус %d", resp.StatusCode)
	}
}

func TestMeAndLogout(t *testing.T) {
	s := apitest.New(t)
	ctx := context.Background()
	c := s.As(t, "marketer")

	me, err := c.GetMeWithResponse(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if me.JSON200 == nil || me.JSON200.Login != "marketer" || me.JSON200.Role != gen.Marketer {
		t.Fatalf("me: %d %s", me.StatusCode(), me.Body)
	}
	if me.HTTPResponse.Header.Get("X-CSRF-Token") == "" {
		t.Error("me не отдаёт X-CSRF-Token")
	}

	out, err := c.LogoutWithResponse(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if out.StatusCode() != http.StatusNoContent {
		t.Fatalf("logout: %d", out.StatusCode())
	}
	again, err := c.GetMeWithResponse(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if again.StatusCode() != http.StatusUnauthorized {
		t.Errorf("me после выхода: %d", again.StatusCode())
	}
}

func TestAdminFromEnvCanLogin(t *testing.T) {
	s := apitest.New(t)
	me, err := s.As(t, "admin").GetMeWithResponse(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if me.JSON200 == nil || me.JSON200.Role != gen.Admin {
		t.Errorf("me: %d %s", me.StatusCode(), me.Body)
	}
}

func TestSessionExpires(t *testing.T) {
	s := apitest.New(t)
	c := s.As(t, "jury")
	s.Clock.Advance(13 * 60 * 60 * 1e9)
	me, err := c.GetMeWithResponse(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if me.StatusCode() != http.StatusUnauthorized {
		t.Errorf("просроченная сессия: %d", me.StatusCode())
	}
}
