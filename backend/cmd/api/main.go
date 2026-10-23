// Command api — сервер «Контура удержания»: собирает модули за контрактом api/openapi.yaml.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"example.com/app/internal/app"
	"example.com/app/internal/auth"
	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/db"
	"example.com/app/internal/platform/events"
	"example.com/app/internal/source"
)

// newMux — маршруты без базы (тесты маршрутов).
func newMux() *http.ServeMux {
	return app.Routes(nil, events.New(), clock.System{}, auth.New(nil, clock.System{}, auth.Config{}))
}

// newHandler собирает полный обработчик без базы (тесты маршрутов).
func newHandler() http.Handler {
	return app.New(nil, events.New(), clock.System{}, auth.New(nil, clock.System{}, auth.Config{}))
}

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
	return c, nil
}

func main() {
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

	authAPI := auth.New(pool, clock.System{}, auth.ConfigFromEnv())
	if err := authAPI.OnStart(ctx); err != nil {
		return err
	}
	bus := events.New()
	if err := source.OnStart(ctx, pool, bus, clock.System{}, os.Getenv("SEED_ORGANIZERS") == "true"); err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.addr,
		Handler:           app.New(pool, bus, clock.System{}, authAPI),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("api: слушаю %s", cfg.addr)
	return srv.ListenAndServe()
}
