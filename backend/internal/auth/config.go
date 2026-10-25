package auth

import (
	"os"
	"time"
)

// Config — настройки модуля; ConfigFromEnv читает их из окружения.
type Config struct {
	// AdminLogin и AdminPassword — администратор; без обоих учётка не создаётся.
	AdminLogin    string
	AdminPassword string
	// DemoPasswords — пароли демо-учёток по роли (marketer, analyst, jury); пустой — учётки нет.
	DemoPasswords map[string]string
	// MetricsToken открывает только /metrics, SourceToken — только PUT /source/snapshot и POST /source/events.
	MetricsToken string
	SourceToken  string
	// CookieSecure ставит флаг Secure на cookie сессии.
	CookieSecure bool
	// TrustedProxy разрешает брать IP клиента из X-Forwarded-For.
	TrustedProxy bool
	// SessionCacheTTL — сколько проверенная сессия живёт в кэше процесса; 0 выключает кэш.
	SessionCacheTTL time.Duration
	// GuestEnabled открывает POST /auth/guest: вход без пароля с ролью viewer (только просмотр).
	GuestEnabled bool
	// OIDC — вход через внешнего провайдера (Keycloak); без Issuer и ClientID SSO выключен.
	OIDC OIDCConfig
}

// ConfigFromEnv читает ADMIN_LOGIN, ADMIN_PASSWORD, DEMO_{MARKETER,ANALYST,JURY}_PASSWORD,
// GUEST_LOGIN_ENABLED, METRICS_TOKEN, SOURCE_TOKEN, COOKIE_SECURE, TRUSTED_PROXY, OIDC_ISSUER_URL, OIDC_DISCOVERY_URL, OIDC_CLIENT_ID,
// OIDC_CLIENT_SECRET, OIDC_REDIRECT_URL, SESSION_CACHE_TTL (по умолчанию 5s, 0 — без кэша).
func ConfigFromEnv() Config {
	return Config{
		AdminLogin:    os.Getenv("ADMIN_LOGIN"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		DemoPasswords: map[string]string{
			RoleMarketer: os.Getenv("DEMO_MARKETER_PASSWORD"),
			RoleAnalyst:  os.Getenv("DEMO_ANALYST_PASSWORD"),
			RoleJury:     os.Getenv("DEMO_JURY_PASSWORD"),
		},
		MetricsToken:    os.Getenv("METRICS_TOKEN"),
		SourceToken:     os.Getenv("SOURCE_TOKEN"),
		CookieSecure:    os.Getenv("COOKIE_SECURE") == "true",
		TrustedProxy:    os.Getenv("TRUSTED_PROXY") == "true",
		SessionCacheTTL: sessionCacheTTLFromEnv(),
		GuestEnabled:    os.Getenv("GUEST_LOGIN_ENABLED") == "true",
		OIDC: OIDCConfig{
			Issuer:       os.Getenv("OIDC_ISSUER_URL"),
			DiscoveryURL: os.Getenv("OIDC_DISCOVERY_URL"),
			ClientID:     os.Getenv("OIDC_CLIENT_ID"),
			ClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
			RedirectURL:  os.Getenv("OIDC_REDIRECT_URL"),
		},
	}
}

const defaultSessionCacheTTL = 5 * time.Second

// sessionCacheTTLFromEnv читает SESSION_CACHE_TTL как длительность Go; пусто, мусор или
// отрицательное значение — значение по умолчанию.
func sessionCacheTTLFromEnv() time.Duration {
	d, err := time.ParseDuration(os.Getenv("SESSION_CACHE_TTL"))
	if err != nil || d < 0 {
		return defaultSessionCacheTTL
	}
	return d
}
