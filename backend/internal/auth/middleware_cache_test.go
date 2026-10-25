package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func get(h http.Handler, token string) int {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/customers", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

// countDB считает походы за сессией в БД.
func countDB(api *AuthAPI) *atomic.Int64 {
	var n atomic.Int64
	inner := api.svc.load
	api.svc.load = func(ctx context.Context, hash string) (sessionRecord, error) {
		n.Add(1)
		return inner(ctx, hash)
	}
	return &n
}

func TestMiddlewareSessionCache(t *testing.T) {
	t.Run("повторный запрос не ходит в БД", func(t *testing.T) {
		api, _, sess, h := middlewareStand(t, Config{SessionCacheTTL: 5 * time.Second})
		n := countDB(api)
		for i := 0; i < 3; i++ {
			if code := get(h, sess.Token); code != http.StatusNoContent {
				t.Fatalf("запрос %d: статус %d", i, code)
			}
		}
		if n.Load() != 1 {
			t.Fatalf("запросов к БД %d, ждали 1", n.Load())
		}
	})

	t.Run("выход: следующий запрос получает 401", func(t *testing.T) {
		api, _, sess, h := middlewareStand(t, Config{SessionCacheTTL: 5 * time.Second})
		_ = countDB(api)
		if code := get(h, sess.Token); code != http.StatusNoContent {
			t.Fatalf("статус %d", code)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sess.Token})
		req.Header.Set("X-CSRF-Token", sess.CSRFToken)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("выход: статус %d", rec.Code)
		}
		if code := get(h, sess.Token); code != http.StatusUnauthorized {
			t.Fatalf("после выхода статус %d, ждали 401", code)
		}
	})

	t.Run("кэш не переживает срок сессии", func(t *testing.T) {
		api, clk, sess, h := middlewareStand(t, Config{SessionCacheTTL: 24 * time.Hour})
		_ = countDB(api)
		if code := get(h, sess.Token); code != http.StatusNoContent {
			t.Fatalf("статус %d", code)
		}
		clk.Advance(sessionTTL)
		if code := get(h, sess.Token); code != http.StatusUnauthorized {
			t.Fatalf("после expires_at статус %d, ждали 401", code)
		}
	})

	t.Run("права проверяются на каждый запрос из кэша", func(t *testing.T) {
		api, _, sess, h := middlewareStand(t, Config{SessionCacheTTL: 5 * time.Second})
		n := countDB(api)
		get(h, sess.Token) // прогрев кэша
		req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sess.Token})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("роль с сессией на служебной операции: статус %d, ждали 403", rec.Code)
		}
		if n.Load() != 1 {
			t.Fatalf("запросов к БД %d, ждали 1 (второй запрос из кэша)", n.Load())
		}
	})
}

// База пропала: сессия из кэша работает, промах кэша — 503 с Retry-After, не 500.
func TestMiddlewareDBDown(t *testing.T) {
	api, _, sess, h := middlewareStand(t, Config{SessionCacheTTL: 5 * time.Second})
	if code := get(h, sess.Token); code != http.StatusNoContent {
		t.Fatalf("прогрев кэша: статус %d", code)
	}
	api.svc.load = func(context.Context, string) (sessionRecord, error) {
		return sessionRecord{}, syscall.ECONNREFUSED
	}
	if code := get(h, sess.Token); code != http.StatusNoContent {
		t.Fatalf("сессия из кэша при упавшей БД: статус %d", code)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/customers", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "незнакомый-токен"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("промах кэша: статус %d, Retry-After %q", rec.Code, rec.Header().Get("Retry-After"))
	}
}

// Зависшая база (pause): load не возвращается сам, запрос обязан уложиться в dbWait и получить 503.
func TestMiddlewareDBHangs(t *testing.T) {
	api, _, _, h := middlewareStand(t, Config{SessionCacheTTL: 5 * time.Second})
	api.svc.load = func(ctx context.Context, _ string) (sessionRecord, error) {
		<-ctx.Done()
		return sessionRecord{}, ctx.Err()
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/customers", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "незнакомый-токен"})
	rec := httptest.NewRecorder()
	start := time.Now()
	h.ServeHTTP(rec, req)
	if d := time.Since(start); d > dbWait+time.Second {
		t.Fatalf("ответ через %v, ждали не больше %v", d, dbWait)
	}
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("статус %d, Retry-After %q", rec.Code, rec.Header().Get("Retry-After"))
	}
}
