package docs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// CSP запрещает встроенные скрипты, поэтому страница Swagger UI подключает только файлы.
func TestDocsPageHasNoInlineScript(t *testing.T) {
	rec := httptest.NewRecorder()
	New().Docs(rec, httptest.NewRequest(http.MethodGet, "/api/v1/docs", nil))
	body := rec.Body.String()
	if strings.Contains(body, "<script>") {
		t.Errorf("встроенный скрипт на странице docs")
	}
	if !strings.Contains(body, "/api/v1/docs/assets/init.js") {
		t.Errorf("init.js не подключён")
	}
}
