// Command api — сервер «Контура удержания»: собирает модули за контрактом api/openapi.yaml.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/app/internal/analytics"
	"example.com/app/internal/auth"
	"example.com/app/internal/model"
	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/db"
	"example.com/app/internal/platform/docs"
	"example.com/app/internal/platform/events"
	"example.com/app/internal/platform/httpx"
	"example.com/app/internal/platform/webui"
	"example.com/app/internal/source"
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

func ping(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(map[string]string{"ping": "pong"}); err != nil {
		log.Printf("ping: %v", err)
	}
}

// server собирает сгенерированный интерфейс из обработчиков модулей. Модули не знают друг
// о друге; каждый реализует только свои операции.
type server struct {
	*auth.AuthAPI
	*source.SourceAPI
	*analytics.AnalyticsAPI
	*model.ModelAPI
	*docs.DocsAPI
}

// Healthz, Version, Ping и PingHead — служебные операции платформы без префикса /api/v1.
func (server) Healthz(w http.ResponseWriter, r *http.Request, _ gen.HealthzParams) { healthz(w, r) }

func (server) Version(w http.ResponseWriter, r *http.Request, _ gen.VersionParams) { version(w, r) }

func (server) Ping(w http.ResponseWriter, r *http.Request, _ gen.PingParams) { ping(w, r) }

func (server) PingHead(w http.ResponseWriter, r *http.Request, _ gen.PingHeadParams) { ping(w, r) }

// problemOnBadRequest отвечает на ошибку разбора параметров запроса как problem+json.
func problemOnBadRequest(w http.ResponseWriter, r *http.Request, err error) {
	httpx.WriteProblem(w, r, http.StatusBadRequest, err.Error())
}

// newRoutes собирает маршруты контракта. pool может быть nil (тесты без базы): пока модули —
// заглушки, они базой не пользуются.
func newRoutes(pool *pgxpool.Pool, bus *events.Bus, clk clock.Clock) *http.ServeMux {
	mux := http.NewServeMux()
	gen.HandlerWithOptions(server{
		AuthAPI:      auth.New(pool, clk),
		SourceAPI:    source.New(pool, bus, clk),
		AnalyticsAPI: analytics.New(pool, bus, clk),
		ModelAPI:     model.New(pool, bus, clk),
		DocsAPI:      docs.New(),
	}, gen.StdHTTPServerOptions{BaseRouter: mux, ErrorHandlerFunc: problemOnBadRequest})
	mux.Handle("GET /api/v1/docs/assets/", docs.Assets())
	mux.Handle("GET /{$}", webui.Handler())
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
		// Генерируемая обёртка разбирает заголовок как UUID: подменяем его нормализованным.
		r.Header.Set("X-Request-Id", id)
		next.ServeHTTP(w, r)
	})
}

// newMux — маршруты без базы (тесты маршрутов).
func newMux() *http.ServeMux { return newRoutes(nil, events.New(), clock.System{}) }

// newHandler собирает полный обработчик без базы (тесты маршрутов).
func newHandler() http.Handler {
	return buildHandler(nil, events.New(), clock.System{})
}

func buildHandler(pool *pgxpool.Pool, bus *events.Bus, clk clock.Clock) http.Handler {
	return withRequestID(auth.Middleware(newRoutes(pool, bus, clk)))
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

	if err := auth.OnStart(ctx); err != nil {
		return err
	}
	if err := source.OnStart(ctx); err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.addr,
		Handler:           buildHandler(pool, events.New(), clock.System{}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("api: слушаю %s", cfg.addr)
	return srv.ListenAndServe()
}
