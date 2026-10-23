// Package auth — вход по логину и паролю, сессии и права ролей. вход через OIDC.
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

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"

	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/httpx"
)

// AuthAPI обслуживает пути /auth/* и проверяет доступ ко всем остальным.
type AuthAPI struct {
	svc  *service
	cfg  Config
	oidc *oidcClient
}

// New создаёт обработчик модуля. pool может быть nil (тесты без базы): сессий тогда нет.
func New(pool *pgxpool.Pool, clk clock.Clock, cfg Config) *AuthAPI {
	a := &AuthAPI{svc: newService(pool, clk, cfg), cfg: cfg}
	if cfg.OIDC.enabled() {
		a.oidc = &oidcClient{cfg: cfg.OIDC}
	}
	return a
}

type ctxKey struct{}

func identityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(ctxKey{}).(Identity)
	return id, ok
}

// LoginFrom возвращает логин вошедшего пользователя из контекста запроса; без сессии — пусто.
func LoginFrom(ctx context.Context) string {
	id, _ := identityFrom(ctx)
	return id.Login
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
		log.Printf("auth: вход: %v", err)
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "ошибка входа")
		return
	}
	a.svc.clearFailures(key)
	a.setSessionCookie(w, sess)
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

// setSessionCookie ставит cookie сессии; общая для входа по паролю и через SSO.
func (a *AuthAPI) setSessionCookie(w http.ResponseWriter, sess Session) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: sess.Token, Path: "/", Expires: sess.ExpiresAt,
		HttpOnly: true, Secure: a.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func (a *AuthAPI) flowCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name: oidcFlowCookie, Value: value, Path: oidcFlowPath, MaxAge: maxAge,
		HttpOnly: true, Secure: a.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	}
}

// OidcLogin — GET /api/v1/auth/oidc/login: перенаправляет на провайдера с PKCE, state и nonce.
func (a *AuthAPI) OidcLogin(w http.ResponseWriter, r *http.Request) {
	if a.oidc == nil {
		httpx.NotImplemented(w, r)
		return
	}
	conf, _, err := a.oidc.oauth(r.Context())
	if err != nil {
		log.Printf("auth: %v", err)
		httpx.WriteProblem(w, r, http.StatusBadGateway, "провайдер SSO недоступен")
		return
	}
	state, err1 := randomToken()
	nonce, err2 := randomToken()
	if err := errors.Join(err1, err2); err != nil {
		log.Printf("auth: %v", err)
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "ошибка входа")
		return
	}
	verifier := oauth2.GenerateVerifier()
	http.SetCookie(w, a.flowCookie(state+"."+nonce+"."+verifier, 600))
	url := conf.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), oidc.Nonce(nonce))
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, url, http.StatusFound)
}

// OidcCallback — GET /api/v1/auth/oidc/callback: обменивает код, проверяет ID-токен и роль,
// заводит ту же сессию, что и вход по паролю.
func (a *AuthAPI) OidcCallback(w http.ResponseWriter, r *http.Request, params gen.OidcCallbackParams) {
	if a.oidc == nil {
		httpx.NotImplemented(w, r)
		return
	}
	c, err := r.Cookie(oidcFlowCookie)
	http.SetCookie(w, a.flowCookie("", -1))
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "вход через SSO не начат")
		return
	}
	parts := strings.SplitN(c.Value, ".", 3)
	if len(parts) != 3 || params.State == nil || params.Code == nil || *params.Code == "" ||
		!tokenEquals(*params.State, parts[0]) {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "неверный state или нет кода")
		return
	}
	nonce, verifier := parts[1], parts[2]

	conf, verifierID, err := a.oidc.oauth(r.Context())
	if err != nil {
		log.Printf("auth: %v", err)
		httpx.WriteProblem(w, r, http.StatusBadGateway, "провайдер SSO недоступен")
		return
	}
	tok, err := conf.Exchange(r.Context(), *params.Code, oauth2.VerifierOption(verifier))
	if err != nil {
		log.Printf("auth: обмен кода: %v", err)
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "вход через SSO не удался")
		return
	}
	raw, _ := tok.Extra("id_token").(string)
	idToken, err := verifierID.Verify(r.Context(), raw)
	if err != nil || !tokenEquals(idToken.Nonce, nonce) {
		log.Printf("auth: ID-токен: %v", err)
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "вход через SSO не удался")
		return
	}
	var claims struct {
		Username    string `json:"preferred_username"`
		RealmAccess struct {
			Roles []string `json:"roles"`
		} `json:"realm_access"`
		Roles []string `json:"roles"`
	}
	if err := idToken.Claims(&claims); err != nil {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "вход через SSO не удался")
		return
	}
	role, err := roleFromClaims(claims.RealmAccess.Roles, claims.Roles)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusForbidden, "у пользователя нет роли для панели")
		return
	}
	name := claims.Username
	if name == "" {
		name = idToken.Subject
	}
	sess, err := a.svc.ssoLogin(r.Context(), ssoLoginPrefix+name, role)
	if err != nil {
		log.Printf("auth: вход SSO %q: %v", name, err)
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "ошибка входа")
		return
	}
	a.setSessionCookie(w, sess)
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, "/", http.StatusFound)
}
