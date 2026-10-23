package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDCConfig — подключение к провайдеру. Пустые Issuer или ClientID отключают SSO.
type OIDCConfig struct {
	Issuer string
	// DiscoveryURL — откуда читать discovery, если он не совпадает с публичным Issuer (Compose).
	DiscoveryURL string
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
		src := c.cfg.Issuer
		if c.cfg.DiscoveryURL != "" {
			ctx = oidc.InsecureIssuerURLContext(ctx, c.cfg.Issuer)
			src = c.cfg.DiscoveryURL
		}
		p, err := oidc.NewProvider(ctx, src)
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
