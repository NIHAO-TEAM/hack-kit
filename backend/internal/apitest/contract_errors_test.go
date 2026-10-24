package apitest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"

	"example.com/app/internal/apitest"
)

// validateAgainstSpec проверяет, что код ответа описан у пути в api/openapi.yaml,
// а тело и заголовки ему соответствуют.
func validateAgainstSpec(t *testing.T, req *http.Request, resp *http.Response) {
	t.Helper()
	raw, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := openapi3.NewLoader().LoadFromData(raw)
	if err != nil {
		t.Fatal(err)
	}
	doc.Servers = nil
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatal(err)
	}
	route, params, err := router.FindRoute(req)
	if err != nil {
		t.Fatalf("путь %s %s не описан в контракте: %v", req.Method, req.URL.Path, err)
	}
	body, _ := io.ReadAll(resp.Body)
	in := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request: req, PathParams: params, Route: route,
			Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
		},
		Status:  resp.StatusCode,
		Header:  resp.Header,
		Options: &openapi3filter.Options{IncludeResponseStatus: true},
	}
	in.SetBodyBytes(body)
	if err := openapi3filter.ValidateResponse(context.Background(), in); err != nil {
		t.Errorf("%s %s → %d не соответствует контракту: %v\nтело: %s", req.Method, req.URL.Path, resp.StatusCode, err, body)
	}
}

func send(t *testing.T, s *apitest.Stand, method, path string, body []byte, mod func(*http.Request)) (*http.Request, *http.Response) {
	t.Helper()
	req, err := http.NewRequest(method, s.URL+path, bytes.NewReader(body))
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
	return req, resp
}

func loginBody(password string) []byte {
	b, _ := json.Marshal(map[string]string{"login": "jury", "password": password})
	return b
}

func TestErrorResponsesMatchContract(t *testing.T) {
	t.Run("429 на перебор пароля", func(t *testing.T) {
		s := apitest.New(t)
		for i := 0; i < 5; i++ {
			send(t, s, http.MethodPost, "/api/v1/auth/login", loginBody("wrong"), nil)
			s.Clock.Advance(10 * time.Second)
		}
		req, resp := send(t, s, http.MethodPost, "/api/v1/auth/login", loginBody("wrong"), nil)
		if resp.StatusCode != http.StatusTooManyRequests {
			t.Fatalf("статус %d, ждали 429", resp.StatusCode)
		}
		validateAgainstSpec(t, req, resp)
	})

	t.Run("403 по cookie без CSRF", func(t *testing.T) {
		s := apitest.New(t)
		_, lg := send(t, s, http.MethodPost, "/api/v1/auth/login", loginBody(apitest.Password("jury")), nil)
		var cookie *http.Cookie
		for _, c := range lg.Cookies() {
			if c.Name == "session" {
				cookie = c
			}
		}
		if cookie == nil {
			t.Fatal("вход не выдал cookie")
		}
		req, resp := send(t, s, http.MethodPatch, "/api/v1/model/settings", []byte("{}"), func(r *http.Request) { r.AddCookie(cookie) })
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("статус %d, ждали 403", resp.StatusCode)
		}
		validateAgainstSpec(t, req, resp)
	})

	t.Run("слишком большой снимок", func(t *testing.T) {
		t.Setenv("SNAPSHOT_MAX_BYTES", "1024")
		s := apitest.New(t)
		big := append([]byte(`{"pad":"`), bytes.Repeat([]byte("x"), 4096)...)
		big = append(big, `"}`...)
		req, resp := send(t, s, http.MethodPut, "/api/v1/source/snapshot", big, func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer "+apitest.SourceToken)
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("статус %d, ждали 400", resp.StatusCode)
		}
		validateAgainstSpec(t, req, resp)
	})
}
