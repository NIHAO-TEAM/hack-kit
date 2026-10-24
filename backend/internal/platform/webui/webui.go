// Package webui раздаёт собранный фронт на «/» с откатом на index.html для маршрутов SPA.
// Фронт встраивается из dist/ при сборке образа; в репозитории там лежит страница-заглушка.
package webui

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strings"

	"example.com/app/internal/platform/httpx"
)

//go:embed all:dist
var embedded embed.FS

// reserved — пути API и служебные: на них откат на index.html не действует, неизвестный путь — 404.
var reserved = []string{"/api/", "/healthz", "/version", "/ping", "/metrics"}

// Handler обслуживает всё, что не занято другими маршрутами (регистрируется как "GET /").
func Handler() http.Handler {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		log.Fatalf("webui: %v", err)
	}
	return handler(sub)
}

func handler(root fs.FS) http.Handler {
	files := http.FileServerFS(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := path.Clean(r.URL.Path)
		for _, prefix := range reserved {
			if p == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(p, prefix) {
				httpx.WriteProblem(w, r, http.StatusNotFound, "путь не найден")
				return
			}
		}
		if p != "/" {
			if f, err := root.Open(strings.TrimPrefix(p, "/")); err == nil {
				info, statErr := f.Stat()
				closeErr := f.Close()
				if statErr == nil && closeErr == nil && !info.IsDir() {
					if strings.HasPrefix(p, "/assets/") {
						w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
					}
					files.ServeHTTP(w, r)
					return
				}
			}
		}
		// Маршрут SPA или корень: index.html без кэша, чтобы новая сборка подхватывалась.
		w.Header().Set("Cache-Control", "no-cache")
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		files.ServeHTTP(w, r2)
	})
}
