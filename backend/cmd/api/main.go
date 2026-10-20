// Command api — заглушка сервера для проверки процесса hack-kit.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/google/uuid"
)

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		log.Printf("healthz: %v", err)
	}
}

func version(w http.ResponseWriter, _ *http.Request) {
	v := os.Getenv("APP_VERSION")
	if v == "" {
		v = "dev"
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"version": v}); err != nil {
		log.Printf("version: %v", err)
	}
}

// pingCount — сколько раз вызвали /ping с момента старта.
var pingCount int

func ping(w http.ResponseWriter, _ *http.Request) {
	pingCount++
	w.Header().Set("X-Ping-Count", strconv.Itoa(pingCount))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(map[string]string{"ping": "pong"}); err != nil {
		log.Printf("ping: %v", err)
	}
}

func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /version", version)
	mux.HandleFunc("GET /ping", ping)
	return mux
}

// withRequestID гарантирует заголовок X-Request-Id в каждом ответе: корректный
// UUID из запроса возвращается нормализованным, иначе генерируется новый.
func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.NewString()
		if parsed, err := uuid.Parse(r.Header.Get("X-Request-Id")); err == nil {
			id = parsed.String()
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r)
	})
}

func newHandler() http.Handler {
	return withRequestID(newMux())
}

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: newHandler(),
	}

	log.Printf("api: слушаю %s", addr)
	log.Fatal(srv.ListenAndServe())
}
