package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"example.com/app/internal/auth/db"
	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/httpx"
)

// OIDCConfig — подключение к провайдеру. Пустые Issuer или ClientID отключают SSO.
type OIDCConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

func (c OIDCConfig) enabled() bool {
	return c.Issuer != "" && c.ClientID != "" && c.RedirectURL != ""
}

const (
	oidcFlowCookie = "oidc_flow"
	oidcFlowPath   = "/api/v1/auth/oidc"
	// ssoLoginPrefix отделяет SSO-учётки от парольных: логин из провайдера не может занять чужую.
	ssoLoginPrefix = "sso:"
)

// ssoRoles — роли, которые можно получить через SSO. jury и admin входят только по паролю.
var ssoRoles = []string{RoleMarketer, RoleAnalyst}

var errNoSSORole = errors.New("auth: у пользователя нет роли для панели")

// oidcClient лениво подключается к провайдеру: Keycloak может подняться позже сервера.
type oidcClient struct {
	cfg OIDCConfig

	mu       sync.Mutex
	provider *oidc.Provider
}

func (c *oidcClient) oauth(ctx context.Context) (*oauth2.Config, *oidc.IDTokenVerifier, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.provider == nil {
		p, err := oidc.NewProvider(ctx, c.cfg.Issuer)
		if err != nil {
			return nil, nil, fmt.Errorf("auth: провайдер OIDC: %w", err)
		}
		c.provider = p
	}
	conf := &oauth2.Config{
		ClientID:     c.cfg.ClientID,
		ClientSecret: c.cfg.ClientSecret,
		RedirectURL:  c.cfg.RedirectURL,
		Endpoint:     c.provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile"},
	}
	return conf, c.provider.Verifier(&oidc.Config{ClientID: c.cfg.ClientID}), nil
}

// roleFromClaims выбирает роль панели из claim провайдера: realm_access.roles (Keycloak)
// или плоский roles. Нет подходящей — отказ.
func roleFromClaims(realmRoles, roles []string) (string, error) {
	have := map[string]bool{}
	for _, r := range realmRoles {
		have[r] = true
	}
	for _, r := range roles {
		have[r] = true
	}
	for _, r := range ssoRoles {
		if have[r] {
			return r, nil
		}
	}
	return "", errNoSSORole
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

// ssoLogin находит или заводит SSO-учётку с ролью из провайдера и открывает сессию.
func (s *service) ssoLogin(ctx context.Context, login, role string) (Session, error) {
	if s.q == nil {
		return Session{}, ErrNoSession
	}
	u, err := s.q.UpsertSSOUser(ctx, db.UpsertSSOUserParams{Login: login, Role: role})
	if err != nil {
		return Session{}, fmt.Errorf("auth: SSO-учётка %s: %w", login, err)
	}
	return s.issueSession(ctx, u.ID, u.Login, u.Role)
}
