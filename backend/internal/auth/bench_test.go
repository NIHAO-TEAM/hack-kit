package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/db"
	"example.com/app/internal/platform/dbtest"
)

// benchMiddleware гоняет запрос с данными через Middleware: сессия по токену, права, обработчик.
func benchMiddleware(b *testing.B, cfg Config) {
	dsn := dbtest.New(b)
	ctx := context.Background()
	if err := db.Up(ctx, dsn); err != nil {
		b.Fatalf("миграции: %v", err)
	}
	pool, err := db.Open(ctx, dsn)
	if err != nil {
		b.Fatalf("база: %v", err)
	}
	b.Cleanup(pool.Close)

	clk := clock.NewFake(time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC))
	cfg.DemoPasswords = map[string]string{RoleMarketer: "pw-marketer"}
	api := New(pool, clk, cfg)
	if err := api.OnStart(ctx); err != nil {
		b.Fatalf("учётки: %v", err)
	}
	sess, err := api.svc.login(ctx, RoleMarketer, "pw-marketer")
	if err != nil {
		b.Fatalf("вход: %v", err)
	}

	mux := http.NewServeMux()
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.Handle("GET /api/v1/customers", ok)
	h := api.Middleware(mux, mux)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/customers", nil)
			req.Header.Set("Authorization", "Bearer "+sess.Token)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusNoContent {
				b.Errorf("статус %d", rec.Code)
				return
			}
		}
	})
}

// BenchmarkMiddlewareDB — каждый запрос ходит в БД за сессией (кэш выключен).
func BenchmarkMiddlewareDB(b *testing.B) { benchMiddleware(b, Config{}) }
