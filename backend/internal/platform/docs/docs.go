// Package docs отдаёт контракт и Swagger UI. Файлы UI и спека вшиты в бинарь, CDN не нужен.
package docs

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"

	"example.com/app/internal/platform/api/gen"
)

// openapi.yaml — копия api/openapi.yaml, её обновляет make generate.
//
//go:embed openapi.yaml
var spec []byte

//go:embed assets
var assetsFS embed.FS

const page = `<!doctype html>
<html lang="ru">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Контур удержания — API</title>
<link rel="icon" href="/api/v1/docs/assets/favicon-32x32.png">
<link rel="stylesheet" href="/api/v1/docs/assets/swagger-ui.css">
</head>
<body>
<div id="swagger-ui"></div>
<script src="/api/v1/docs/assets/swagger-ui-bundle.js"></script>
<script src="/api/v1/docs/assets/init.js"></script>
</body>
</html>
`

// DocsAPI обслуживает /api/v1/docs, /api/v1/openapi.yml и /api/v1/openapi.json.
type DocsAPI struct{}

// New создаёт обработчик.
func New() *DocsAPI { return &DocsAPI{} }

// Docs — GET /api/v1/docs.
func (*DocsAPI) Docs(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write([]byte(page)); err != nil {
		log.Printf("docs: %v", err)
	}
}

// OpenapiYaml — GET /api/v1/openapi.yml: файл контракта как есть.
func (*DocsAPI) OpenapiYaml(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	if _, err := w.Write(spec); err != nil {
		log.Printf("docs: %v", err)
	}
}

// OpenapiJson — GET /api/v1/openapi.json: тот же контракт в JSON.
func (*DocsAPI) OpenapiJson(w http.ResponseWriter, _ *http.Request) {
	swagger, err := gen.GetSwagger()
	if err != nil {
		http.Error(w, fmt.Sprintf("контракт: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(swagger); err != nil {
		log.Printf("docs: %v", err)
	}
}

// Assets — GET /api/v1/docs/assets/*: файлы Swagger UI.
func Assets() http.Handler {
	sub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		panic(err) // каталог вшит при сборке
	}
	return http.StripPrefix("/api/v1/docs/assets/", http.FileServerFS(sub))
}
