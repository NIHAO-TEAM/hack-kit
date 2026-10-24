// Command api — сервер «Контура удержания»: собирает модули за контрактом api/openapi.yaml.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"example.com/app/internal/app"
	"example.com/app/internal/auth"
	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/db"
	"example.com/app/internal/platform/events"
	"example.com/app/internal/platform/httpx"
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
	drain       time.Duration // пауза после снятия готовности до Shutdown
	timeout     time.Duration // сколько Shutdown ждёт идущие запросы
}

// Умолчания остановки; terminationGracePeriodSeconds в Helm больше их суммы.
const (
	defaultDrain   = 5 * time.Second
	defaultTimeout = 20 * time.Second
)

// envDuration читает длительность из окружения (формат time.ParseDuration).
func envDuration(name string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(name)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("%s: нужна длительность вида 5s, получено %q", name, v)
	}
	return d, nil
}

func loadConfig() (config, error) {
	c := config{addr: os.Getenv("HTTP_ADDR"), databaseURL: os.Getenv("DATABASE_URL")}
	var err error
	if c.drain, err = envDuration("SHUTDOWN_DRAIN", defaultDrain); err != nil {
		return config{}, err
	}
	if c.timeout, err = envDuration("SHUTDOWN_TIMEOUT", defaultTimeout); err != nil {
		return config{}, err
	}
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
	// Корневой контекст: SIGTERM/SIGINT останавливают воркеры пересчёта, биллинга и эмулятора.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
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

	var workers sync.WaitGroup
	// Воркер пересчёта: первый расчёт идёт сразу, дальше по source.changed и model.changed.
	recalc := app.NewRecalc(pool, bus, clock.System{})
	recalcStopped := recalc.Start(ctx)
	workers.Add(1)
	go func() { defer workers.Done(); <-recalcStopped }()

	lc := httpx.NewLifecycle()
	lc.Ready = func() bool { return ready(recalc.Engine.Latest() != nil, recalc.Engine.LastError()) }
	lc.Ping = pool.Ping
	opts := []app.Option{app.WithCollectors(lc.Collector(), authAPI.Collector())}

	// Опрос REST-биллинга включается, только если задан BILLING_URL.
	if bc := source.AdapterConfigFromEnv(); bc != nil {
		billing := source.NewAdapter(pool, bus, clock.System{}, *bc)
		workers.Add(1)
		go func() { defer workers.Done(); billing.Run(ctx) }()
		opts = append(opts, app.WithBilling(billing))
		log.Printf("api: опрос биллинга каждые %s", bc.Interval)
	}

	srv := newServer(cfg.addr, app.New(pool, bus, clock.System{}, authAPI, recalc, opts...), lc)
	ln, err := net.Listen("tcp", cfg.addr)
	if err != nil {
		return err
	}
	log.Printf("api: слушаю %s", cfg.addr)
	err = serve(ctx, srv, ln, lc, cfg.drain, cfg.timeout)
	// Сервер остановлен: ждём воркеры (ctx уже отменён), и только потом закрывается пул (defer).
	workers.Wait()
	return err
}

// newServer оборачивает обработчик приложения: /readyz отвечает мимо сессии и счётчика запросов,
// остальные запросы считаются, а контекст потока событий отменяется при остановке.
func newServer(addr string, h http.Handler, lc *httpx.Lifecycle) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /readyz", lc.ReadyHandler)
	mux.Handle("/", lc.Track(h))
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	srv.RegisterOnShutdown(lc.CancelStreams)
	return srv
}

// serve обслуживает ln, пока ctx не отменён. Порядок остановки: /readyz отвечает 503 →
// пауза drain (балансировщик снимает под) → Shutdown с таймаутом: идущие запросы дорабатывают,
// потоки SSE закрываются. Вернёт nil при штатной остановке.
func serve(ctx context.Context, srv *http.Server, ln net.Listener, lc *httpx.Lifecycle, drain, timeout time.Duration) error {
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	started := time.Now()
	lc.BeginShutdown()
	log.Printf("api: остановка, запросов в работе: %d", lc.InflightAtShutdown())
	time.Sleep(drain)
	sctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := srv.Shutdown(sctx)
	if err != nil {
		_ = srv.Close() // таймаут: оставшиеся соединения закрываются принудительно
	}
	log.Printf("api: остановлен за %s (ошибка: %v)", time.Since(started).Round(time.Millisecond), err)
	return err
}

// ready — готов ли под принимать трафик: есть результат пересчёта, либо считать нечего (снимок ещё
// не загружали — чистая установка). Без второго условия свежий под на пустой базе никогда не
// становится готовым, и `helm install --wait` падает по таймауту, хотя снимок грузят уже в готовый сервис.
func ready(hasResult bool, lastErr error) bool {
	return hasResult || errors.Is(lastErr, source.ErrNoSnapshot)
}
