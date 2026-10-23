// Package webui раздаёт собранный фронт на «/». Пока заглушка: на «/» текстовая страница
// «фронт не собран», остальные пути — 404. Наполняет изменение compose-stand.
package webui

import (
	"log"
	"net/http"
)

// Handler обслуживает «/» (регистрируется как "GET /{$}").
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if _, err := w.Write([]byte("фронт не собран\n")); err != nil {
			log.Printf("webui: %v", err)
		}
	})
}
