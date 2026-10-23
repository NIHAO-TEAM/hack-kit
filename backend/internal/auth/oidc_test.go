package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"example.com/app/internal/apitest"
	"example.com/app/internal/auth"
	"example.com/app/internal/platform/api/gen"
)

const ssoClientID = "panel"

// fakeIdP — минимальный провайдер OIDC: discovery, JWKS и обмен кода на ID-токен.
type fakeIdP struct {
	srv   *httptest.Server
	key   *rsa.PrivateKey
	roles []string
	// nonce запоминается при заходе на /authorize.
	nonce string
}

func newFakeIdP(t *testing.T, roles ...string) *fakeIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p := &fakeIdP{key: key, roles: roles}
	mux := http.NewServeMux()
	p.srv = httptest.NewServer(mux)
	t.Cleanup(p.srv.Close)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer": p.srv.URL, "authorization_endpoint": p.srv.URL + "/authorize",
			"token_endpoint": p.srv.URL + "/token", "jwks_uri": p.srv.URL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
			{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"},
		}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("code") != "good-code" || r.Form.Get("code_verifier") == "" {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: key, KeyID: "k1"}}, nil)
		if err != nil {
			t.Error(err)
			return
		}
		claims := map[string]any{"preferred_username": "secure", "realm_access": map[string]any{"roles": p.roles}, "nonce": p.nonce}
		std := jwt.Claims{Issuer: p.srv.URL, Subject: "u1", Audience: jwt.Audience{ssoClientID}, Expiry: jwt.NewNumericDate(time.Now().Add(time.Hour)), IssuedAt: jwt.NewNumericDate(time.Now())}
		raw, err := jwt.Signed(signer).Claims(std).Claims(claims).Serialize()
		if err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "a", "token_type": "Bearer", "expires_in": 3600, "id_token": raw})
	})
	return p
}

func ssoStand(t *testing.T, idp *fakeIdP) *apitest.Stand {
	return apitest.NewWithConfig(t, func(c *auth.Config) {
		c.OIDC = auth.OIDCConfig{Issuer: idp.srv.URL, ClientID: ssoClientID, ClientSecret: "s", RedirectURL: "http://app.test/api/v1/auth/oidc/callback"}
	})
}

var noRedirect = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

// startSSO заходит на /oidc/login и возвращает state и cookie потока.
func startSSO(t *testing.T, s *apitest.Stand, idp *fakeIdP) (string, *http.Cookie) {
	t.Helper()
	resp, err := noRedirect.Get(s.URL + "/api/v1/auth/oidc/login")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("login: статус %d", resp.StatusCode)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil || !strings.HasPrefix(loc.String(), idp.srv.URL+"/authorize") {
		t.Fatalf("Location %q", resp.Header.Get("Location"))
	}
	q := loc.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" || q.Get("state") == "" || q.Get("nonce") == "" {
		t.Fatalf("нет PKCE/state/nonce: %v", q)
	}
	idp.nonce = q.Get("nonce")
	for _, c := range resp.Cookies() {
		if c.Name == "oidc_flow" {
			return q.Get("state"), c
		}
	}
	t.Fatal("нет cookie потока")
	return "", nil
}

func callback(t *testing.T, s *apitest.Stand, state, code string, flow *http.Cookie) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, s.URL+"/api/v1/auth/oidc/callback?code="+code+"&state="+state, nil)
	if flow != nil {
		req.AddCookie(flow)
	}
	resp, err := noRedirect.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func TestSSOLoginCreatesSessionWithRole(t *testing.T) {
	idp := newFakeIdP(t, "offline_access", "analyst")
	s := ssoStand(t, idp)
	state, flow := startSSO(t, s, idp)
	resp := callback(t, s, state, "good-code", flow)
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/" {
		t.Fatalf("статус %d, Location %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	c := sessionCookie(resp)
	if c == nil || !c.HttpOnly {
		t.Fatalf("cookie сессии: %+v", c)
	}
	req, _ := http.NewRequest(http.MethodGet, s.URL+"/api/v1/auth/me", nil)
	req.AddCookie(c)
	me, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = me.Body.Close() }()
	var u gen.User
	if err := json.NewDecoder(me.Body).Decode(&u); err != nil {
		t.Fatal(err)
	}
	if me.StatusCode != http.StatusOK || u.Role != gen.Analyst || u.Login != "sso:secure" {
		t.Errorf("me: %d %+v", me.StatusCode, u)
	}
}

func TestSSOPasswordLoginStillWorks(t *testing.T) {
	s := ssoStand(t, newFakeIdP(t, "analyst"))
	if resp := login(t, s, "analyst", apitest.Password("analyst")); resp.StatusCode != http.StatusOK {
		t.Fatalf("статус %d", resp.StatusCode)
	}
}

func TestSSOSSOAccountHasNoPassword(t *testing.T) {
	idp := newFakeIdP(t, "marketer")
	s := ssoStand(t, idp)
	state, flow := startSSO(t, s, idp)
	callback(t, s, state, "good-code", flow)
	for _, pw := range []string{"!", "x"} {
		if resp := login(t, s, "sso:secure", pw); resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("пароль %q: статус %d", pw, resp.StatusCode)
		}
	}
}

func TestSSORejects(t *testing.T) {
	idp := newFakeIdP(t, "marketer")
	s := ssoStand(t, idp)
	state, flow := startSSO(t, s, idp)
	for name, tc := range map[string]struct {
		state, code string
		flow        *http.Cookie
		want        int
	}{
		"чужой state": {"other", "good-code", flow, http.StatusBadRequest},
		"нет cookie":  {state, "good-code", nil, http.StatusBadRequest},
		"плохой код":  {state, "bad-code", flow, http.StatusUnauthorized},
		"пустой код":  {state, "", flow, http.StatusBadRequest},
	} {
		t.Run(name, func(t *testing.T) {
			resp := callback(t, s, tc.state, tc.code, tc.flow)
			if resp.StatusCode != tc.want || sessionCookie(resp) != nil {
				t.Errorf("статус %d, cookie %v", resp.StatusCode, sessionCookie(resp))
			}
		})
	}
}

func TestSSOWrongNonceRejected(t *testing.T) {
	idp := newFakeIdP(t, "marketer")
	s := ssoStand(t, idp)
	state, flow := startSSO(t, s, idp)
	idp.nonce = "forged"
	if resp := callback(t, s, state, "good-code", flow); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("статус %d", resp.StatusCode)
	}
}

func TestSSORoleRequired(t *testing.T) {
	for _, roles := range [][]string{nil, {"admin", "jury", "offline_access"}} {
		idp := newFakeIdP(t, roles...)
		s := ssoStand(t, idp)
		state, flow := startSSO(t, s, idp)
		resp := callback(t, s, state, "good-code", flow)
		if resp.StatusCode != http.StatusForbidden || sessionCookie(resp) != nil {
			t.Errorf("роли %v: статус %d, cookie %v", roles, resp.StatusCode, sessionCookie(resp))
		}
	}
}

func TestSSODisabledWithoutConfig(t *testing.T) {
	s := apitest.New(t)
	resp, err := noRedirect.Get(s.URL + "/api/v1/auth/oidc/login")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("статус %d", resp.StatusCode)
	}
}

func getAuthConfig(t *testing.T, s *apitest.Stand) gen.AuthConfig {
	t.Helper()
	resp, err := http.Get(s.URL + "/api/v1/auth/config")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("статус %d", resp.StatusCode)
	}
	var cfg gen.AuthConfig
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestAuthConfigWithoutOIDC(t *testing.T) {
	if getAuthConfig(t, apitest.New(t)).SsoEnabled {
		t.Error("без OIDC_* ssoEnabled должен быть false")
	}
}

func TestAuthConfigWithOIDC(t *testing.T) {
	if !getAuthConfig(t, ssoStand(t, newFakeIdP(t, "marketer"))).SsoEnabled {
		t.Error("с OIDC_* ssoEnabled должен быть true")
	}
}
