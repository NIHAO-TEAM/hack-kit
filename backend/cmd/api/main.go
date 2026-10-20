// Command api — заглушка сервера для проверки процесса hack-kit.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
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

func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /version", version)
	return mux
}

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           newMux(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("api: слушаю %s", addr)
	log.Fatal(srv.ListenAndServe())
}
