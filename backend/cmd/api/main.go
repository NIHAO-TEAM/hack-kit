// Command api — сервер «Контура удержания»: собирает модули за контрактом api/openapi.yaml.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"example.com/app/internal/app"
	"example.com/app/internal/auth"
	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/db"
	"example.com/app/internal/platform/events"
	"example.com/app/internal/source"
)

// newMux — маршруты без базы (тесты маршрутов).
func newMux() *http.ServeMux { return app.Routes(nil, events.New(), clock.System{}) }

// newHandler собирает полный обработчик без базы (тесты маршрутов).
func newHandler() http.Handler { return app.New(nil, events.New(), clock.System{}) }

// config — настройки из окружения.
type config struct {
	addr        string
	databaseURL string
}

func loadConfig() (config, error) {
	c := config{addr: os.Getenv("HTTP_ADDR"), databaseURL: os.Getenv("DATABASE_URL")}
	if c.addr == "" {
		c.addr = os.Getenv("ADDR")
	}
	if c.addr == "" {
		c.addr = ":8080"
	}
	if c.databaseURL == "" {
		return config{}, errors.New("DATABASE_URL не задан")
	}
	if os.Getenv("ADMIN_PASSWORD") == "" {
		return config{}, errors.New("ADMIN_PASSWORD не задан: укажите пароль администратора в .env")
	}
	return c, nil
}

// healthcheck опрашивает /healthz своего сервера: в образе distroless нет curl и shell.
func healthcheck() error {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthz: статус %d", resp.StatusCode)
	}
	return nil
}

func main() {
	check := flag.Bool("healthcheck", false, "проверить /healthz запущенного сервера и выйти")
	flag.Parse()
	if *check {
		if err := healthcheck(); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ctx := context.Background()
	if err := db.Up(ctx, cfg.databaseURL); err != nil {
		return err
	}
	pool, err := db.Open(ctx, cfg.databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := auth.OnStart(ctx); err != nil {
		return err
	}
	if err := source.OnStart(ctx); err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.addr,
		Handler:           app.New(pool, events.New(), clock.System{}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("api: слушаю %s", cfg.addr)
	return srv.ListenAndServe()
}
