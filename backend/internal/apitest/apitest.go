// Package apitest поднимает весь API тем же конструктором, что cmd/api, на отдельной базе
// и отдаёт сгенерированный клиент. Тесты эндпоинтов ходят только через него.
package apitest

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"example.com/app/internal/app"
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
}

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
	srv := httptest.NewServer(app.New(pool, bus, clk))
	t.Cleanup(srv.Close)

	client, err := gen.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatalf("apitest: клиент: %v", err)
	}
	return &Stand{Client: client, URL: srv.URL, Clock: clk, Bus: bus}
}
