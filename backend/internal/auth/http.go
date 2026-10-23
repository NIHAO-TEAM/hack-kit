// Package auth — вход по логину и паролю, сессии и права ролей. OIDC (слой 2) — заглушка.
package auth

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/httpx"
)

// AuthAPI обслуживает пути /auth/* и проверяет доступ ко всем остальным.
type AuthAPI struct {
	svc *service
	cfg Config
}

// New создаёт обработчик модуля. pool может быть nil (тесты без базы): сессий тогда нет.
func New(pool *pgxpool.Pool, clk clock.Clock, cfg Config) *AuthAPI {
	return &AuthAPI{svc: newService(pool, clk, cfg), cfg: cfg}
}

type ctxKey struct{}

func identityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(ctxKey{}).(Identity)
	return id, ok
}

const sessionCookie = "session"

// bearerToken возвращает токен из заголовка Authorization.
func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

func tokenEquals(got, want string) bool {
	return want != "" && subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func safeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

// Middleware проверяет сессию и права на каждый запрос. Операцию определяет шаблон маршрута
// mux: маршрут вне таблицы operations закрыт без сессии, неизвестный путь отдаёт mux (404).
func (a *AuthAPI) Middleware(mux *http.ServeMux, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pattern := mux.Handler(r)
		if pattern == "" {
			next.ServeHTTP(w, r)
			return
		}
		op, known := operations[pattern]
		if known && op.access == accessPublic {
			next.ServeHTTP(w, r)
			return
		}

		bearer := bearerToken(r)
		// Служебные токены открывают ровно одну операцию и не дают сессии.
		if known && ((op.id == "putSnapshot" && tokenEquals(bearer, a.cfg.SourceToken)) ||
			((op.id == "getMetrics" || op.id == "getMetricsAlias") && tokenEquals(bearer, a.cfg.MetricsToken))) {
			next.ServeHTTP(w, r)
			return
		}

		token, fromCookie := bearer, false
		if token == "" {
			if c, err := r.Cookie(sessionCookie); err == nil {
				token, fromCookie = c.Value, true
			}
		}
		id, err := a.svc.authenticate(r.Context(), token)
		if errors.Is(err, ErrNoSession) {
			httpx.WriteProblem(w, r, http.StatusUnauthorized, "нужен вход")
			return
		}
		if err != nil {
			log.Printf("auth: %v", err)
			httpx.WriteProblem(w, r, http.StatusInternalServerError, "ошибка проверки сессии")
			return
		}
		if fromCookie && !safeMethod(r.Method) && !tokenEquals(r.Header.Get("X-CSRF-Token"), id.CSRFToken) {
			httpx.WriteProblem(w, r, http.StatusForbidden, "нужен заголовок X-CSRF-Token")
			return
		}
		if !known || !can(id.Role, op.id) {
			httpx.WriteProblem(w, r, http.StatusForbidden, "недостаточно прав")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, id)))
	})
}

// OnStart создаёт учётки из окружения.
func (a *AuthAPI) OnStart(ctx context.Context) error {
	if a.svc.q == nil {
		return nil
	}
	return a.svc.ensureUsers(ctx)
}

func (a *AuthAPI) clientIP(r *http.Request) string {
	if a.cfg.TrustedProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first, _, _ := strings.Cut(xff, ",")
			return strings.TrimSpace(first)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func toUser(id Identity) gen.User {
	return gen.User{Id: id.UserID, Login: id.Login, Role: gen.Role(id.Role)}
}

// Login — POST /api/v1/auth/login.
func (a *AuthAPI) Login(w http.ResponseWriter, r *http.Request) {
	var req gen.LoginRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil || req.Login == "" || req.Password == "" {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "нужны login и password")
		return
	}
	key := req.Login + "\x00" + a.clientIP(r)
	if wait, blocked := a.svc.reserveAttempt(key); blocked {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
		httpx.WriteProblem(w, r, http.StatusTooManyRequests, "слишком много попыток входа")
		return
	}
	sess, err := a.svc.login(r.Context(), req.Login, req.Password)
	if errors.Is(err, ErrBadCredentials) {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "неверный логин или пароль")
		return
	}
	if err != nil {
		log.Printf("auth: вход %q: %v", req.Login, err)
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "ошибка входа")
		return
	}
	a.svc.clearFailures(key)
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: sess.Token, Path: "/", Expires: sess.ExpiresAt,
		HttpOnly: true, Secure: a.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(gen.LoginResult{User: toUser(sess.Identity), Token: &sess.Token, CsrfToken: &sess.CSRFToken}); err != nil {
		log.Printf("auth: ответ входа: %v", err)
	}
}

// Logout — POST /api/v1/auth/logout.
func (a *AuthAPI) Logout(w http.ResponseWriter, r *http.Request) {
	id, _ := identityFrom(r.Context())
	if err := a.svc.logout(r.Context(), id.TokenHash); err != nil {
		log.Printf("auth: выход: %v", err)
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "ошибка выхода")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: a.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

// GetMe — GET /api/v1/auth/me. CSRF-токен сессии отдаётся заголовком X-CSRF-Token:
// в схеме User для него поля нет, контракт этим изменением не трогаем.
func (a *AuthAPI) GetMe(w http.ResponseWriter, r *http.Request) {
	id, _ := identityFrom(r.Context())
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-CSRF-Token", id.CSRFToken)
	if err := json.NewEncoder(w).Encode(toUser(id)); err != nil {
		log.Printf("auth: ответ me: %v", err)
	}
}

// OidcLogin — GET /api/v1/auth/oidc/login (слой 2).
func (*AuthAPI) OidcLogin(w http.ResponseWriter, r *http.Request) { httpx.NotImplemented(w, r) }

// OidcCallback — GET /api/v1/auth/oidc/callback (слой 2).
func (*AuthAPI) OidcCallback(w http.ResponseWriter, r *http.Request, _ gen.OidcCallbackParams) {
	httpx.NotImplemented(w, r)
}
