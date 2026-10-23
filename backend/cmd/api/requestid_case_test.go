package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDHeaderNameCase(t *testing.T) {
	const id = "f47ac10b-58cc-4372-8567-0e02b2c3d479"
	srv := httptest.NewServer(newHandler())
	defer srv.Close()

	tests := []struct {
		name string
		key  string
	}{
		{"нижний регистр", "x-request-id"},
		{"верхний регистр", "X-REQUEST-ID"},
		{"смешанный регистр", "x-ReQuEsT-iD"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, srv.URL+"/healthz", nil)
			if err != nil {
				t.Fatal(err)
			}
			// Прямая запись в map: Header.Set канонизировал бы имя, и в сеть
			// ушёл бы не тот регистр, который проверяем.
			req.Header[tt.key] = []string{id}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if got := resp.Header.Get("X-Request-Id"); got != id {
				t.Errorf("X-Request-Id = %q, ждали %q", got, id)
			}
		})
	}
}
