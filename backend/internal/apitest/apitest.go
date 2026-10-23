// Package apitest поднимает весь API тем же конструктором, что cmd/api, на отдельной базе
// и отдаёт сгенерированный клиент. Тесты эндпоинтов ходят только через него.
package apitest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/app/internal/analytics"
	"example.com/app/internal/app"
	"example.com/app/internal/auth"
	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/db"
	"example.com/app/internal/platform/dbtest"
	"example.com/app/internal/platform/events"
)

// Stand — запущенный API.
type Stand struct {
	// Client — клиент, сгенерированный из api/openapi.yaml.
	Client *gen.ClientWithResponses
	// URL — адрес сервера.
	URL string
	// Clock — фейковые часы сервера; тесты двигают время через Advance.
	Clock *clock.Fake
	// Bus — шина событий сервера.
	Bus *events.Bus
	// Pool — пул соединений к базе стенда (для проверок состояния в тестах).
	Pool *pgxpool.Pool
	feed *lazyFeed
	// Handler — тот же обработчик без сети, для тестов через httptest.ResponseRecorder.
	Handler http.Handler
}

// Учётки и токены, которые стенд создаёт при старте.
const (
	AdminLogin    = "admin"
	AdminPassword = "admin-pass"
	MetricsToken  = "metrics-token"
	SourceToken   = "source-token"
)

// Password — пароль демо-учётки роли на стенде.
func Password(role string) string { return "pw-" + role }

// New применяет миграции к чистой базе, запускает сервер и закрывает всё в t.Cleanup.
// Без TEST_DATABASE_URL тест пропускается.
func New(t testing.TB) *Stand {
	t.Helper()
	dsn := dbtest.New(t)
	ctx := context.Background()
	if err := db.Up(ctx, dsn); err != nil {
		t.Fatalf("apitest: миграции: %v", err)
	}
	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("apitest: база: %v", err)
	}
	t.Cleanup(pool.Close)

	clk := clock.NewFake(time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC))
	bus := events.New()
	cfg := auth.Config{
		AdminLogin:    AdminLogin,
		AdminPassword: AdminPassword,
		DemoPasswords: map[string]string{
			auth.RoleMarketer: Password(auth.RoleMarketer),
			auth.RoleAnalyst:  Password(auth.RoleAnalyst),
			auth.RoleJury:     Password(auth.RoleJury),
		},
		MetricsToken: MetricsToken,
		SourceToken:  SourceToken,
	}
	authAPI := auth.New(pool, clk, cfg)
	if err := authAPI.OnStart(ctx); err != nil {
		t.Fatalf("apitest: учётки: %v", err)
	}
	feed := &lazyFeed{}
	handler := app.New(pool, bus, clk, authAPI, feed)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client, err := gen.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatalf("apitest: клиент: %v", err)
	}
	return &Stand{Pool: pool, Client: client, URL: srv.URL, Clock: clk, Bus: bus, feed: feed, Handler: handler}
}

// As входит под ролью (marketer, analyst, jury) или логином admin и возвращает клиента,
// который шлёт токен сессии как Authorization: Bearer.
func (s *Stand) As(t testing.TB, role string) *gen.ClientWithResponses {
	t.Helper()
	login, password := role, Password(role)
	if role == auth.RoleAdmin {
		login, password = AdminLogin, AdminPassword
	}
	res, err := s.Client.LoginWithResponse(context.Background(), gen.LoginJSONRequestBody{Login: login, Password: password})
	if err != nil {
		t.Fatalf("apitest: вход %s: %v", role, err)
	}
	if res.JSON200 == nil || res.JSON200.Token == nil {
		t.Fatalf("apitest: вход %s: %d %s", role, res.StatusCode(), res.Body)
	}
	return s.clientWithToken(t, *res.JSON200.Token)
}

// Anonymous возвращает клиента без сессии.
func (s *Stand) Anonymous(t testing.TB) *gen.ClientWithResponses {
	t.Helper()
	return s.clientWithToken(t, "")
}

// WithBearer возвращает клиента с произвольным Bearer-токеном (служебные токены).
func (s *Stand) WithBearer(t testing.TB, token string) *gen.ClientWithResponses {
	t.Helper()
	return s.clientWithToken(t, token)
}

func (s *Stand) clientWithToken(t testing.TB, token string) *gen.ClientWithResponses {
	t.Helper()
	c, err := gen.NewClientWithResponses(s.URL, gen.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		return nil
	}))
	if err != nil {
		t.Fatalf("apitest: клиент: %v", err)
	}
	return c
}

// lazyFeed отдаёт портфель воркера, когда тест его запустил; до этого расчёта нет.
type lazyFeed struct{ recalc atomic.Pointer[app.Recalc] }

// Overview отдаёт сводку воркера, когда тест его запустил.
func (f *lazyFeed) Overview() *analytics.Overview {
	if r := f.recalc.Load(); r != nil {
		return r.Overview()
	}
	return nil
}

// QualityChecks отдаёт проверки качества данных воркера; до его запуска — nil.
func (f *lazyFeed) QualityChecks(version int64) []gen.QualityCheck {
	if r := f.recalc.Load(); r != nil {
		return r.QualityChecks(version)
	}
	return nil
}

func (f *lazyFeed) Portfolio() *analytics.Portfolio {
	if r := f.recalc.Load(); r != nil {
		return r.Portfolio()
	}
	return nil
}

// StartRecalc запускает воркер пересчёта и подключает его к API: клиенты отвечают данными
// последнего расчёта. Воркер не стартует сам, чтобы тесты, чистящие таблицы, не ловили с ним
// взаимные блокировки. Останавливается в t.Cleanup до закрытия пула.
func (s *Stand) StartRecalc(t testing.TB) *app.Recalc {
	t.Helper()
	r := app.NewRecalc(s.Pool, s.Bus, s.Clock)
	ctx, cancel := context.WithCancel(context.Background())
	done := r.Start(ctx)
	t.Cleanup(func() {
		cancel()
		<-done
	})
	s.feed.recalc.Store(r)
	return r
}
