package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func get(path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	newHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestDocsOpenWithoutLogin(t *testing.T) {
	rec := get("/api/v1/docs")
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "SwaggerUIBundle") || strings.Contains(body, "https://") {
		t.Errorf("страница должна запускать Swagger UI без внешних ссылок")
	}
	for _, asset := range []string{"swagger-ui.css", "swagger-ui-bundle.js"} {
		if r := get("/api/v1/docs/assets/" + asset); r.Code != http.StatusOK || r.Body.Len() < 1000 {
			t.Errorf("%s: статус %d, размер %d", asset, r.Code, r.Body.Len())
		}
	}
}

func TestOpenAPIYamlIsContractFile(t *testing.T) {
	want, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	rec := get("/api/v1/openapi.yml")
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d", rec.Code)
	}
	if rec.Body.String() != string(want) {
		t.Error("/openapi.yml отличается от api/openapi.yaml: запустите make generate")
	}
}

func TestOpenAPIJson(t *testing.T) {
	rec := get("/api/v1/openapi.json")
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d", rec.Code)
	}
	var doc struct {
		OpenAPI string                     `json:"openapi"`
		Paths   map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.OpenAPI == "" || doc.Paths["/api/v1/auth/login"] == nil || doc.Paths["/healthz"] == nil {
		t.Errorf("в JSON нет ожидаемых путей: %s %d путей", doc.OpenAPI, len(doc.Paths))
	}
}

func TestFrontPlaceholderOnlyAtRoot(t *testing.T) {
	if r := get("/"); r.Code != http.StatusOK || !strings.Contains(r.Body.String(), "фронт не собран") {
		t.Errorf("/: %d %q", r.Code, r.Body.String())
	}
	if r := get("/nope"); r.Code != http.StatusNotFound {
		t.Errorf("/nope: %d", r.Code)
	}
}
