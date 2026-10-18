// Command api — заглушка сервера для проверки процесса hack-kit.
package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

func newMux() *http.ServeMux {
	return http.NewServeMux()
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
