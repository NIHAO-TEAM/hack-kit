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
func newMux() *http.ServeMux {
	return app.Routes(nil, events.New(), clock.System{}, auth.New(nil, clock.System{}, auth.Config{}), nil)
}

// newHandler собирает полный обработчик без базы (тесты маршрутов).
func newHandler() http.Handler {
	return app.New(nil, events.New(), clock.System{}, auth.New(nil, clock.System{}, auth.Config{}), nil)
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

	authAPI := auth.New(pool, clock.System{}, auth.ConfigFromEnv())
	if err := authAPI.OnStart(ctx); err != nil {
		return err
	}
	bus := events.New()
	if err := source.OnStart(ctx, pool, bus, clock.System{}, os.Getenv("SEED_ORGANIZERS") == "true"); err != nil {
		return err
	}

	// Воркер пересчёта: первый расчёт идёт сразу, дальше по source.changed и model.changed.
	recalc := app.NewRecalc(pool, bus, clock.System{})
	recalc.Start(ctx)

	srv := &http.Server{
		Addr:              cfg.addr,
		Handler:           app.New(pool, bus, clock.System{}, authAPI, recalc),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("api: слушаю %s", cfg.addr)
	return srv.ListenAndServe()
}
