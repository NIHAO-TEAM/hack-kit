// Package auth — вход, сессии и права. Пока заглушка: каждая операция отвечает 501.
// Наполняет изменение auth-sessions.
package auth

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/httpx"
)

// AuthAPI обслуживает пути /auth/*.
type AuthAPI struct {
	pool *pgxpool.Pool
	clk  clock.Clock
}

// New создаёт обработчик модуля.
func New(pool *pgxpool.Pool, clk clock.Clock) *AuthAPI {
	return &AuthAPI{pool: pool, clk: clk}
}

// Middleware проверяет сессию и права на каждый запрос. Заглушка пропускает всё.
func Middleware(next http.Handler) http.Handler { return next }

// OnStart создаёт учётки из окружения. Заглушка ничего не делает.
func OnStart(context.Context) error { return nil }

// Login — POST /api/v1/auth/login.
func (*AuthAPI) Login(w http.ResponseWriter, r *http.Request) { httpx.NotImplemented(w, r) }

// Logout — POST /api/v1/auth/logout.
func (*AuthAPI) Logout(w http.ResponseWriter, r *http.Request) { httpx.NotImplemented(w, r) }

// GetMe — GET /api/v1/auth/me.
func (*AuthAPI) GetMe(w http.ResponseWriter, r *http.Request) { httpx.NotImplemented(w, r) }

// OidcLogin — GET /api/v1/auth/oidc/login (слой 2).
func (*AuthAPI) OidcLogin(w http.ResponseWriter, r *http.Request) { httpx.NotImplemented(w, r) }

// OidcCallback — GET /api/v1/auth/oidc/callback (слой 2).
func (*AuthAPI) OidcCallback(w http.ResponseWriter, r *http.Request, _ gen.OidcCallbackParams) {
	httpx.NotImplemented(w, r)
}
