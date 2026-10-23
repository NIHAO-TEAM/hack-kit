// Package app собирает HTTP-сервер: модули за сгенерированным интерфейсом контракта.
// Им пользуются cmd/api и тестовая обвязка apitest, поэтому тесты ходят в тот же сервер, что и прод.
package app

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/app/internal/analytics"
	"example.com/app/internal/auth"
	"example.com/app/internal/model"
	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/clock"
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

// Routes собирает маршруты контракта. pool может быть nil (тесты без базы): пока модули —
// заглушки, они базой не пользуются.
func Routes(pool *pgxpool.Pool, bus *events.Bus, clk clock.Clock, authAPI *auth.AuthAPI, rc *Recalc) *http.ServeMux {
	mux := http.NewServeMux()
	gen.HandlerWithOptions(server{
		AuthAPI:      authAPI,
		SourceAPI:    source.New(pool, bus, clk),
		AnalyticsAPI: analytics.New(pool, bus, clk).WithOverview(rc.Overview),
		ModelAPI:     model.New(pool, bus, clk),
		DocsAPI:      docs.New(),
	}, gen.StdHTTPServerOptions{BaseRouter: mux, ErrorHandlerFunc: problemOnBadRequest})
	mux.Handle("GET /api/v1/docs/assets/", docs.Assets())
	mux.Handle("GET /", webui.Handler())
	return mux
}

// WithRequestID гарантирует заголовок X-Request-Id в каждом ответе: корректный
// UUID из запроса возвращается нормализованным, иначе генерируется новый.
func WithRequestID(next http.Handler) http.Handler {
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

// New собирает полный обработчик: идентификатор запроса, проверка сессии, маршруты.
func New(pool *pgxpool.Pool, bus *events.Bus, clk clock.Clock, authAPI *auth.AuthAPI, rc *Recalc) http.Handler {
	mux := Routes(pool, bus, clk, authAPI, rc)
	return WithRequestID(authAPI.Middleware(mux, mux))
}
