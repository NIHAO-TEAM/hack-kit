package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func serve(t *testing.T, target string) *httptest.ResponseRecorder {
	t.Helper()
	root := fstest.MapFS{
		"index.html":   {Data: []byte("<html>INDEX</html>")},
		"assets/a.js":  {Data: []byte("console.log(1)")},
		"healthz/x.js": {Data: []byte("не должен отдаваться")},
	}
	rec := httptest.NewRecorder()
	handler(root).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestHandler(t *testing.T) {
	tests := []struct {
		name, path string
		code       int
		body       string
	}{
		{"корень", "/", 200, "INDEX"},
		{"маршрут SPA", "/queue", 200, "INDEX"},
		{"вложенный маршрут SPA", "/clients/42", 200, "INDEX"},
		{"ассет", "/assets/a.js", 200, "console.log(1)"},
		{"API не перехватывается", "/api/v1/x", 404, ""},
		{"служебный подпуть", "/healthz/extra", 404, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(t, tt.path)
			if rec.Code != tt.code {
				t.Fatalf("статус %d, ждали %d", rec.Code, tt.code)
			}
			if tt.body != "" && !strings.Contains(rec.Body.String(), tt.body) {
				t.Fatalf("тело %q не содержит %q", rec.Body.String(), tt.body)
			}
		})
	}
}

func TestAssetCache(t *testing.T) {
	if got := serve(t, "/assets/a.js").Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Fatalf("Cache-Control ассета: %q", got)
	}
	if got := serve(t, "/queue").Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control index.html: %q", got)
	}
}

func TestEmbeddedStub(t *testing.T) {
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "фронт не собран") {
		t.Fatalf("заглушка: %d %q", rec.Code, rec.Body.String())
	}
}
